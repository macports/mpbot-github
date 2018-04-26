package webhook

import (
	"bufio"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/macports/mpbot-github/ci/logger/constants"
)

type TravisWebhookPayload struct {
	ID                int    `json:"id"`
	Number            string `json:"number"`
	Type              string `json:"type"`
	State             string `json:"state"`
	Status            int    `json:"status"`
	Result            int    `json:"result"`
	StatusMessage     string `json:"status_message"`
	ResultMessage     string `json:"result_message"`
	Duration          int    `json:"duration"`
	BuildURL          string `json:"build_url"`
	Branch            string `json:"branch"`
	PullRequest       bool   `json:"pull_request"`
	PullRequestNumber int    `json:"pull_request_number"`
	PullRequestTitle  string `json:"pull_request_title"`
	Repository        struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		OwnerName string `json:"owner_name"`
	} `json:"repository"`
	Matrix []struct {
		ID       int    `json:"id"`
		ParentID int    `json:"parent_id"`
		Number   string `json:"number"`
		State    string `json:"state"`
		Config   struct {
			Os       string `json:"os"`
			OsxImage string `json:"osx_image"`
		} `json:"config"`
		Status       int  `json:"status"`
		Result       int  `json:"result"`
		AllowFailure bool `json:"allow_failure"`
	} `json:"matrix"`
}

func (receiver *Receiver) handleTravisWebhook(payload TravisWebhookPayload) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}

		if !receiver.testing {
			receiver.wg.Done()
		}
	}()

	if !payload.PullRequest {
		return
	}

	if payload.Repository.OwnerName != "macports" && payload.Repository.OwnerName != "macports-staging" {
		return
	}

	log.Println("PR #" + strconv.Itoa(payload.PullRequestNumber) + " " + payload.ResultMessage + " on Travis CI")

	comment := "[Travis Build #" + payload.Number + "](" + payload.BuildURL + ") " + payload.ResultMessage + ".\n\n"
	timeOut := false
	lintDone := false

	log.Println("Processing " + strconv.Itoa(len(payload.Matrix)) + " job(s)")

	for _, job := range payload.Matrix {
		req, err := http.NewRequest(
			"GET",
			"https://api.travis-ci.org/job/"+strconv.Itoa(job.ID)+"/log",
			nil,
		)
		if err != nil {
			continue
		}

		req.Header.Set("Travis-API-Version", "3")
		req.Header.Set("Accept", "text/plain")

		log.Println("Fetching logs for job #" + strconv.Itoa(job.ID))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		bufReader := bufio.NewReader(resp.Body)

		for {
			line, err := bufReader.ReadString('\n')
			if err != nil {
				break
			}
			if strings.Contains(line, "$ sudo ./runner") {
				break
			}
		}

		body, err := ioutil.ReadAll(bufReader)
		resp.Body.Close()
		if err != nil {
			continue
		}
		bodyStr := string(body)
		bodyStr = strings.Replace(bodyStr, "\n\n\n\r", "\n", -1)
		bodyStr = strings.Replace(bodyStr, "\r", "", -1)

		mr := multipart.NewReader(strings.NewReader(bodyStr), constants.MIMEBoundary)
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Println(err)
				return
			}
			pName := p.FormName()
			content, err := ioutil.ReadAll(p)
			if err == io.ErrUnexpectedEOF {
				if strings.Contains(
					string(content),
					"The job exceeded the maximum time limit for jobs, and has been terminated.",
				) {
					timeOut = true
				}
				break
			}
			if err != nil {
				log.Println(err)
				continue
			}
			if pName == "keep-alive" {
				continue
			}
			if err != nil {
				log.Println(err)
				continue
			}
			if strings.HasPrefix(pName, "port-lint-output-") && len(content) > 0 && !lintDone {
				comment += "<details><summary>Lint results</summary>\n\n```\n" + string(content) + "```\n</details>\n\n<br>\n\n"
				lintDone = true
			}
			if strings.HasSuffix(pName, "-pastebin") {
				pastebinRegex := regexp.MustCompile(`^port-(.*?)(-dep)?-install-output-(success|fail)-pastebin$`)
				pbInfo := pastebinRegex.FindStringSubmatch(pName)
				if pbInfo == nil {
					continue
				}
				comment += "Port " + pbInfo[1]
				if pbInfo[2] == "-dep" {
					comment += "'s dependencies"
				}
				pasteLink := strings.SplitN(string(content), "\n", 2)
				if len(pasteLink) > 0 {
					comment += " **" + pbInfo[3] + "** on " + job.Config.OsxImage + ". [Log](" + pasteLink[0] + ")\n"
				}
			}
		}
	}

	if timeOut {
		comment += "\nThe build timed out."
	}

	receiver.githubClient.CreateComment(
		payload.Repository.OwnerName,
		payload.Repository.Name,
		payload.PullRequestNumber,
		&comment,
	)
}
