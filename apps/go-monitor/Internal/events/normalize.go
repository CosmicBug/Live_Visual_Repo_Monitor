package events

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/model"
)

type ownerPayload struct {
	Login string `json:"login"`
}

type repoPayload struct {
	ID            int64        `json:"id"`
	Name          string       `json:"name"`
	FullName      string       `json:"full_name"`
	DefaultBranch string       `json:"default_branch"`
	HTMLURL       string       `json:"html_url"`
	Owner         ownerPayload `json:"owner"`
}

type installationPayload struct {
	ID int64 `json:"id"`
}

type senderPayload struct {
	Login string `json:"login"`
}

type genericPayload struct {
	Action       string              `json:"action"`
	Repository   repoPayload         `json:"repository"`
	Installation installationPayload `json:"installation"`
	Sender       senderPayload       `json:"sender"`
	Ref          string              `json:"ref"`
	Before       string              `json:"before"`
	After        string              `json:"after"`
	Created      bool                `json:"created"`
	Deleted      bool                `json:"deleted"`
	PullRequest  *struct {
		Number int `json:"number"`
		Head   struct {
			SHA string `json:"sha"`
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"pull_request"`
	CheckRun *struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HeadSHA    string `json:"head_sha"`
	} `json:"check_run"`
	CheckSuite *struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HeadSHA    string `json:"head_sha"`
	} `json:"check_suite"`
	WorkflowRun *struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HeadSHA    string `json:"head_sha"`
	} `json:"workflow_run"`
	Release *struct {
		TagName string `json:"tag_name"`
	} `json:"release"`
}

func Normalize(eventName, deliveryID string, body []byte, receivedAt time.Time) (model.NormalizedEvent, error) {
	var p genericPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return model.NormalizedEvent{}, fmt.Errorf("decode webhook payload: %w", err)
	}
	if p.Repository.ID == 0 {
		return model.NormalizedEvent{}, fmt.Errorf("payload does not contain repository.id")
	}

	repo := model.Repository{
		ID:             p.Repository.ID,
		Owner:          p.Repository.Owner.Login,
		Name:           p.Repository.Name,
		FullName:       p.Repository.FullName,
		DefaultBranch:  p.Repository.DefaultBranch,
		InstallationID: p.Installation.ID,
		HTMLURL:        p.Repository.HTMLURL,
	}
	if repo.FullName == "" && repo.Owner != "" && repo.Name != "" {
		repo.FullName = repo.Owner + "/" + repo.Name
	}

	ev := model.NormalizedEvent{
		DeliveryID:     deliveryID,
		Event:          eventName,
		Action:         p.Action,
		Repository:     repo,
		InstallationID: p.Installation.ID,
		Ref:            p.Ref,
		Branch:         branchFromRef(p.Ref),
		Before:         p.Before,
		After:          p.After,
		Created:        p.Created,
		Deleted:        p.Deleted,
		SenderLogin:    p.Sender.Login,
		ReceivedAt:     receivedAt,
	}

	if p.PullRequest != nil {
		ev.PullRequestNumber = p.PullRequest.Number
		ev.HeadSHA = p.PullRequest.Head.SHA
		ev.Branch = p.PullRequest.Head.Ref
		ev.BaseRef = p.PullRequest.Base.Ref
	}
	if p.CheckRun != nil {
		ev.CheckName = p.CheckRun.Name
		ev.Status = p.CheckRun.Status
		ev.Conclusion = p.CheckRun.Conclusion
		ev.HeadSHA = p.CheckRun.HeadSHA
	}
	if p.CheckSuite != nil {
		ev.Status = p.CheckSuite.Status
		ev.Conclusion = p.CheckSuite.Conclusion
		ev.HeadSHA = p.CheckSuite.HeadSHA
	}
	if p.WorkflowRun != nil {
		ev.WorkflowName = p.WorkflowRun.Name
		ev.Status = p.WorkflowRun.Status
		ev.Conclusion = p.WorkflowRun.Conclusion
		ev.HeadSHA = p.WorkflowRun.HeadSHA
	}
	if p.Release != nil {
		ev.ReleaseTag = p.Release.TagName
	}
	return ev, nil
}

func branchFromRef(ref string) string {
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}
	if strings.HasPrefix(ref, "refs/tags/") {
		return strings.TrimPrefix(ref, "refs/tags/")
	}
	return filepath.Base(ref)
}
