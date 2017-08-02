package db

import (
	"database/sql"
	"errors"
	"os"
	"strings"

	// PostgreSQL driver
	_ "github.com/lib/pq"
)

type Maintainer struct {
	GithubHandle string
	Email        string
}

type PortMaintainer struct {
	Primary        *Maintainer
	Others         []*Maintainer
	NoMaintainer   bool
	OpenMaintainer bool
}

type DBHelper interface {
	GetGitHubHandle(email string) (string, error)
	GetPortMaintainer(port string) (*PortMaintainer, error)
}

func NewDBHelper() (DBHelper, error) {
	// TODO: move os.Getenv to main
	tracDB, err := sql.Open("postgres", os.Getenv("TRAC_DB"))
	if err != nil {
		return nil, err
	}
	err = tracDB.Ping()
	if err != nil {
		return nil, err
	}

	wwwDB, err := sql.Open("postgres", os.Getenv("WWW_DB"))
	if err != nil {
		return nil, err
	}
	err = wwwDB.Ping()
	if err != nil {
		return nil, err
	}

	prDB, err := sql.Open("postgres", os.Getenv("PR_DB"))
	if err != nil {
		return nil, err
	}
	err = prDB.Ping()
	if err != nil {
		return nil, err
	}

	return &sqlDBHelper{
		tracDB: tracDB,
		wwwDB:  wwwDB,
		prDB:   prDB,
	}, nil
}

type sqlDBHelper struct {
	tracDB, wwwDB, prDB *sql.DB
}

func (sqlDB *sqlDBHelper) GetGitHubHandle(email string) (string, error) {
	sid := ""
	err := sqlDB.tracDB.QueryRow("SELECT sid "+
		"FROM trac_macports.session_attribute "+
		"WHERE value = $1 "+
		"AND name = 'email' "+
		"AND authenticated = 1 "+
		"LIMIT 1", email).
		Scan(&sid)
	if err != nil {
		return "", err
	}
	return sid, nil
}

// GetPortMaintainer returns the maintainers of a port
func (sqlDB *sqlDBHelper) GetPortMaintainer(port string) (*PortMaintainer, error) {
	rows, err := sqlDB.wwwDB.Query("SELECT maintainer, is_primary "+
		"FROM public.maintainers "+
		"WHERE portfile = $1", port)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	maintainer := new(PortMaintainer)
	maintainerCursor := ""
	isPrimary := false
	rowExist := false

	for rows.Next() {
		if err := rows.Scan(&maintainerCursor, &isPrimary); err != nil {
			return nil, err
		}
		rowExist = true
		switch maintainerCursor {
		case "nomaintainer":
			maintainer.NoMaintainer = true
			continue
		case "openmaintainer":
			maintainer.OpenMaintainer = true
			continue
		}
		if isPrimary {
			maintainer.Primary = sqlDB.parseMaintainer(maintainerCursor)
		} else {
			maintainer.Others = append(maintainer.Others, sqlDB.parseMaintainer(maintainerCursor))
		}
	}

	if !rowExist {
		return nil, errors.New("port not found")
	}

	return maintainer, nil
}

func (sqlDB *sqlDBHelper) parseMaintainer(maintainerFullString string) *Maintainer {
	maintainer := parseMaintainerString(maintainerFullString)
	if maintainer.GithubHandle == "" && maintainer.Email != "" {
		if handle, err := sqlDB.GetGitHubHandle(maintainer.Email); err == nil {
			maintainer.GithubHandle = handle
		}
	}
	return maintainer
}

func parseMaintainerString(maintainerFullString string) *Maintainer {
	maintainerStrings := strings.Split(maintainerFullString, " ")
	maintainer := new(Maintainer)
	for _, maintainerString := range maintainerStrings {
		if strings.HasPrefix(maintainerString, "@") {
			maintainer.GithubHandle = maintainerString[1:]
		} else if strings.Count(maintainerString, ":") == 1 {
			emailParts := strings.Split(maintainerString, ":")
			maintainer.Email = emailParts[1] + "@" + emailParts[0]
		} else {
			maintainer.Email = maintainerString + "@macports.org"
		}
	}
	return maintainer
}
