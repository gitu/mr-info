# Mr. Info

## Description

Mr. Info is a utility to fetch gitlab merge requests and import the changes into a disconnected jira instance.

## Installation
with go:

```
go get github.com/gitu/mr-info
```

with docker:

```
docker pull ghcr.io/gitu/mr-info
```

## Usage

### disconnected

If the machine running mr-info does not have access to both gitlab and jira, you can run the commands separately.

To fetch merge requests from gitlab - this will create ioFile:

```
mr-info fetch
```

Afterwards you can transfer the ioFile to a machine with access to jira and run:

```
mr-info push
```

### connected

If the machine running mr-info has access to both gitlab and jira, you can run both commands at once:

```
mr-info sync
```

## Configuration

Mr. Info is configured via a yaml file. By default, it looks for a file called `config.yaml` in the current directory.

### Example config

```yaml
gitlab:
  token: xxxx
  url: https://gitlab.com/api/v4
  projects: # gitlab projects to import from
    - /asdf/project1
    - /asdf/project2
  merge_requests:
    # updated_duration: only MRs updated in the last 96 hours will be considered
    updated_duration: "96h"
    # title_regex: regex to extract issue from MR title - the project subgroup is used to filter the targets against, the issue subgroup is used to extract the issue
    title_regex: "[a-zA-Z]+\\((?P<Issue>(?P<Project>[A-Z]+)-[0-9]+)\\):.*"
    # version_regex: regex to extract the released version from the semantic release comment
    version_regex: ":tada: This MR is included in version (?P<Version>[0-9]+\\.[0-9]+\\.[0-9]+) :tada:"
    # version_url_regex: regex to extract the release url from the semantic release comment
    version_url_regex: "The release is available on \\[GitLab release\\]\\((?P<VersionUrl>[^)]+)\\)."
    release_bot_usernames: # usernames of users that are considered for the semantic release comment
      - ggg
jira:
  targets: # jira projects to import into
    - PROJECTA
    - PROJECTB
  token: xxxx
  url: https://jira.atlassian.net
ioFile: output.yaml  # used for disconnected mode
```

