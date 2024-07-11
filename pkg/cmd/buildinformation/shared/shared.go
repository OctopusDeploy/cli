package shared

type WorkItemAsJson struct {
	Id          string `json:"Id"`
	Source      string `json:"Source"`
	Description string `json:"Description"`
}

type CommitAsJson struct {
	Id      string `json:"Id"`
	Comment string `json:"Comment"`
}

type BuildInfoAsJson struct {
	Id               string            `json:"Id"`
	PackageId        string            `json:"PackageId"`
	Version          string            `json:"Version"`
	Branch           string            `json:"Branch"`
	BuildEnvironment string            `json:"BuildEnvironment"`
	VcsCommitNumber  string            `json:"VcsCommitNumber"`
	VcsType          string            `json:"VcsType"`
	VcsRoot          string            `json:"VcsRoot"`
	Commits          []*CommitAsJson   `json:"Commits"`
	WorkItems        []*WorkItemAsJson `json:"WorkItems"`
}
