package main

import (
	"fmt"
	"github.com/charmbracelet/log"
	. "github.com/gitu/mr-info/pkg/logging"
	"github.com/spf13/viper"
	"github.com/xanzy/go-gitlab"
	"gopkg.in/yaml.v3"
	"os"
	"regexp"
	"time"
)

type MergeRequestInfo struct {
	Project              string     `yaml:"project,omitempty"`
	Issue                string     `yaml:"issue,omitempty"`
	Title                string     `yaml:"title,omitempty"`
	State                string     `yaml:"state,omitempty"`
	TadaVersion          string     `yaml:"tada_version,omitempty"`
	VersionUrl           string     `yaml:"version_url,omitempty"`
	MergeRequestUpdateAt *time.Time `yaml:"merge_request_update_at,omitempty"`
	NoteUpdateAt         *time.Time `yaml:"note_update_at,omitempty"`
}

//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen -package gitlab -generate types,client -o pkg/gitlab/gitlab.gen.go openapi-gitlab.yaml
func main() {

	initialize()

	client, err := gitlab.NewClient(viper.GetString("gitlab.token"), gitlab.WithBaseURL(viper.GetString("gitlab.url")))
	if err != nil {
		Fatal("Failed to create client", err)
	}
	selectedProjects, jiraTargets := getTargetProjects(client)
	issuesToMergeRequests := buildIssueMergeRequestMap(selectedProjects, client, jiraTargets)

	out, err := yaml.Marshal(issuesToMergeRequests)
	if err != nil {
		Fatal("Failed to marshal", err)
	}
	err = os.WriteFile(viper.GetString("output"), out, 0644)
	if err != nil {
		Fatal("Failed to write file", err, "file", viper.GetString("output"))
	}
}

func buildIssueMergeRequestMap(selectedProjects []*gitlab.Project, client *gitlab.Client, jiraTargets map[string]string) map[string][]MergeRequestInfo {
	issuesToMergeRequests := map[string][]MergeRequestInfo{}
	viper.SetDefault("gitlab.merge_requests.updated_duration", "24h")
	viper.SetDefault("gitlab.merge_requests.title_regex", `[a-zA-Z]+\((?P<Issue>(?P<Project>[A-Z]+)-[0-9]+)\):.*`)
	viper.SetDefault("gitlab.merge_requests.version_regex", `:tada: This MR is included in version (?P<Version>[0-9]+\.[0-9]+\.[0-9]+) :tada:`)
	viper.SetDefault("gitlab.merge_requests.version_url_regex", `The release is available on \[GitLab release\]\((?P<VersionUrl>[^)]+)\).`)

	botUsernamesList := viper.GetStringSlice("gitlab.merge_requests.release_bot_usernames")
	botUsernames := map[string]string{}
	for _, botUsername := range botUsernamesList {
		botUsernames[botUsername] = botUsername
	}

	re := regexp.MustCompile(viper.GetString("gitlab.merge_requests.title_regex"))
	rev := regexp.MustCompile(viper.GetString("gitlab.merge_requests.version_regex"))
	revu := regexp.MustCompile(viper.GetString("gitlab.merge_requests.version_url_regex"))
	for _, project := range selectedProjects {
		Log.Info("Project", "id", project.ID, "name", project.Name, "path", project.PathWithNamespace)
		mergeRequests := getAllMergeRequests(client, project)

		for _, mergeRequest := range mergeRequests {
			matches := re.FindStringSubmatch(mergeRequest.Title)
			if matches == nil {
				Log.Debug("Merge request does not match", "title", mergeRequest.Title)
				continue
			}
			Log.Info("Matching merge request", "id", mergeRequest.ID, "title", mergeRequest.Title, "state", mergeRequest.State)
			issue := matches[re.SubexpIndex("Issue")]
			jiraProject := matches[re.SubexpIndex("Project")]

			Log.Info("Issue", "project", jiraProject, "issue", issue)
			_, selected := jiraTargets[jiraProject]
			if selected {
				Log.Info("SELECTED", "project", jiraProject, "issue", issue)
			} else {
				Log.Info("IGNORED", "project", jiraProject, "issue", issue)
			}

			notes, _, err := client.Notes.ListMergeRequestNotes(project.ID, mergeRequest.IID, &gitlab.ListMergeRequestNotesOptions{
				ListOptions: gitlab.ListOptions{
					PerPage: 100,
				},
			})
			if err != nil {
				Fatal("Failed to list notes", err)
			}
			version := ""
			versionUrl := ""
			var noteUpdate *time.Time

			for _, note := range notes {
				if _, found := botUsernames[note.Author.Username]; found {
					submatch := rev.FindStringSubmatch(note.Body)
					if submatch != nil {
						version = submatch[rev.SubexpIndex("Version")]
						noteUpdate = note.UpdatedAt

						submatchUrl := revu.FindStringSubmatch(note.Body)
						if submatchUrl != nil {
							versionUrl = submatchUrl[revu.SubexpIndex("VersionUrl")]
						} else {
							log.Warn("no version url found", "note", note.Title)
						}
						log.Debug("found", "version", version)
					}

				}
			}

			issuesToMergeRequests[issue] = append(issuesToMergeRequests[issue], MergeRequestInfo{
				MergeRequestUpdateAt: mergeRequest.UpdatedAt,
				Project:              jiraProject,
				Issue:                issue,
				Title:                mergeRequest.Title,
				State:                mergeRequest.State,
				NoteUpdateAt:         noteUpdate,
				TadaVersion:          version,
				VersionUrl:           versionUrl,
			})
		}
	}
	return issuesToMergeRequests
}

func getTargetProjects(client *gitlab.Client) ([]*gitlab.Project, map[string]string) {
	projectsSelector, selectedProjects := getSelectedProjects(client)
	if len(selectedProjects) == 0 {
		Fatal("No projects selected", nil, "selector", projectsSelector)
	}
	targetJiraProjects := viper.GetStringSlice("jira.targets")
	jiraTargets := map[string]string{}
	for _, targetJiraProject := range targetJiraProjects {
		jiraTargets[targetJiraProject] = targetJiraProject
	}
	return selectedProjects, jiraTargets
}

func getSelectedProjects(client *gitlab.Client) ([]string, []*gitlab.Project) {
	projects := getAllProjects(client)
	projectsSelector := viper.GetStringSlice("gitlab.projects")
	selectedProjects := make([]*gitlab.Project, 0)
	for _, project := range projects {
		if len(projectsSelector) > 0 {
			for _, projectSelector := range projectsSelector {
				if projectSelector == fmt.Sprintf("%d", project.ID) || projectSelector == project.PathWithNamespace {
					selectedProjects = append(selectedProjects, project)
					break
				}
			}
		}
	}
	return projectsSelector, selectedProjects
}

func getAllMergeRequests(client *gitlab.Client, project *gitlab.Project) []*gitlab.MergeRequest {
	mergeRequests := make([]*gitlab.MergeRequest, 0)
	page := 1

	duration := viper.GetDuration("gitlab.merge_requests.updated_duration")
	date := time.Now().Add(-1 * duration)
	for i := 0; i < 1000; i++ {
		options := gitlab.ListProjectMergeRequestsOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
			UpdatedAfter: &date,
		}
		mergeRequestsPage, response, err := client.MergeRequests.ListProjectMergeRequests(project.ID, &options)
		if err != nil {
			Fatal("Failed to list merge requests", err)
		}
		mergeRequests = append(mergeRequests, mergeRequestsPage...)
		if response.NextPage == 0 {
			return mergeRequests
		}
		page = response.NextPage
	}
	Fatal("Too many pages", nil)
	return nil
}

func getAllProjects(client *gitlab.Client) []*gitlab.Project {
	projects := make([]*gitlab.Project, 0)
	page := 1

	for i := 0; i < 1000; i++ {
		options := gitlab.ListProjectsOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		}
		projectsPage, response, err := client.Projects.ListProjects(&options)
		if err != nil {
			Fatal("Failed to list projects", err)
		}
		projects = append(projects, projectsPage...)
		if response.NextPage == 0 {
			return projects
		}
		page = response.NextPage
	}
	Fatal("Too many pages", nil)
	return nil
}

func initialize() {
	LogHandler.SetReportCaller(true)
	viper.SetEnvPrefix("MR_INFO")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/mr-info/")
	viper.AddConfigPath("$HOME/.mr-info")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Warn("No config file found", "error", err)
	}

	if viper.GetBool("debug") {
		LogHandler.SetLevel(log.DebugLevel)
	}

}
