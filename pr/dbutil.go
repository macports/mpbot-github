package pr

import (
	"database/sql"
	"log"
	"strings"

	_ "github.com/lib/pq"
)

type Maintainer struct {
	GithubHandle string
	Email        string
}

type PortMaintainer struct {
	Primary        *Maintainer
	Others         []*Maintainer
	OpenMaintainer bool
	NoMaintainer   bool
}

var tracDB *sql.DB
var wwwDB *sql.DB

// Create connections to DBs
func init() {
	var err error
	// TODO: use real dbname or read from env/flag
	tracDB, err = sql.Open("postgres", "host=/tmp dbname=l2dy")
	if err != nil {
		log.Fatal(err)
	}
	wwwDB, err = sql.Open("postgres", "host=/tmp dbname=l2dy")
	if err != nil {
		log.Fatal(err)
	}
}

func GetGitHubHandle(email string) (string, error) {
	sid := ""
	err := tracDB.QueryRow("SELECT sid "+
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

// GetMaintainer returns the maintainers of a port,
// the primary maintainer is always the first in the slice.
func GetMaintainer(port string) (*PortMaintainer, error) {
	rows, err := wwwDB.Query("SELECT maintainer, is_primary "+
		"FROM public.maintainers "+
		"WHERE portfile = $1", port)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	maintainer := new(PortMaintainer)
	maintainerCursor := ""
	isPrimary := false

	for rows.Next() {
		if err := rows.Scan(&maintainerCursor, &isPrimary); err != nil {
			return nil, err
		}
		if isPrimary {
			maintainer.Primary = parseMaintainer(maintainerCursor)
		} else {
			maintainer.Others = append(maintainer.Others, parseMaintainer(maintainerCursor))
		}
	}

	return maintainer, nil
}

func parseMaintainer(maintainerFullString string) *Maintainer {
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
