package db

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"strings"
	"time"

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

type PullRequest struct {
	Number        int
	Processed     bool
	PendingReview bool
	Maintainers   []string
}

type DBHelper interface {
	GetGitHubHandle(email string) (string, error)
	GetPortMaintainer(port string) (*PortMaintainer, error)
	NewPR(number int, maintainers []string) error
	GetPR(number int) (*PullRequest, error)
	GetTimeoutPRs() ([]*PullRequest, error)
	SetPRProcessed(number int, processed bool) error
	SetPRPendingReview(number int, pendingReview bool) error
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
	_, err = prDB.Exec(`CREATE TABLE IF NOT EXISTS pull_requests
(
	number INT PRIMARY KEY,
	created TIMESTAMP NOT NULL,
	processed BOOLEAN NOT NULL,
	pending_review BOOLEAN NOT NULL,
	maintainers TEXT NOT NULL
);`)
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

	err = rows.Err()
	if err != nil {
		//TODO: log in caller
		log.Println(err)
	}

	if !rowExist {
		return nil, errors.New("port not found")
	}

	return maintainer, nil
}

func (sqlDB *sqlDBHelper) NewPR(number int, maintainers []string) error {
	_, err := sqlDB.prDB.Exec("INSERT INTO pull_requests VALUES ($1, $2, $3, $4, $5)",
		number, time.Now(), false, false, strings.Join(maintainers, " "))
	return err
}

func (sqlDB *sqlDBHelper) GetPR(number int) (*PullRequest, error) {
	pr := new(PullRequest)
	var maintainerString string
	err := sqlDB.prDB.QueryRow(
		"SELECT number, processed, pending_review, maintainers FROM pull_requests WHERE number = $1", number).
		Scan(&pr.Number, &pr.Processed, &pr.PendingReview, &maintainerString)
	if err != nil {
		return nil, err
	}
	pr.Maintainers = strings.Split(maintainerString, " ")
	return pr, nil
}

func (sqlDB *sqlDBHelper) GetTimeoutPRs() ([]*PullRequest, error) {
	var prs []*PullRequest
	rows, err := sqlDB.prDB.Query("SELECT number, processed, pending_review, maintainers "+
		"FROM pull_requests "+
		"WHERE created <= $1 AND pending_review = true", time.Now().AddDate(0, 0, -3))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		pr := new(PullRequest)
		var maintainerString string
		if err := rows.Scan(&pr.Number, &pr.Processed, &pr.PendingReview, &maintainerString); err != nil {
			return nil, err
		}
		pr.Maintainers = strings.Split(maintainerString, " ")
		prs = append(prs, pr)
	}

	err = rows.Err()
	if err != nil {
		log.Println(err)
	}

	return prs, nil
}

func (sqlDB *sqlDBHelper) SetPRProcessed(number int, processed bool) error {
	_, err := sqlDB.prDB.Exec("UPDATE pull_requests SET processed = $1 WHERE number = $2", processed, number)
	return err
}

func (sqlDB *sqlDBHelper) SetPRPendingReview(number int, pendingReview bool) error {
	_, err := sqlDB.prDB.Exec("UPDATE pull_requests SET pending_review = $1 WHERE number = $2", pendingReview, number)
	return err
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
