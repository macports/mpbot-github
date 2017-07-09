package pr

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

type Maintainer struct {
	primary        string
	others         []string
	openMaintainer bool
	noMaintainer   bool
}

var tracDB *sql.DB
var wwwDB *sql.DB

// Create connections to DBs
func init() {
	var err error
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
// TODO: parse multi identity per maintainer and email
func GetMaintainer(port string) (*Maintainer, error) {
	rows, err := wwwDB.Query("SELECT maintainer, is_primary "+
		"FROM public.maintainers "+
		"WHERE portfile = $1", port)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	maintainer := new(Maintainer)
	maintainerCursor := ""
	isPrimary := false

	for rows.Next() {
		if err := rows.Scan(&maintainerCursor, &isPrimary); err != nil {
			return nil, err
		}
		if isPrimary {
			maintainer.primary = maintainerCursor
		} else {
			maintainer.others = append(maintainer.others, maintainerCursor)
		}
	}

	return maintainer, nil
}
