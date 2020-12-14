package main

import (
	"encoding/json"
	"fmt"
	"github.com/samcontesse/gitlab-merge-request-resource"
	"github.com/samcontesse/gitlab-merge-request-resource/common"
	"github.com/samcontesse/gitlab-merge-request-resource/out"
	"github.com/xanzy/go-gitlab"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
)

func main() {

	var request out.Request
	var message string

	if len(os.Args) < 2 {
		println("usage: " + os.Args[0] + " <destination>")
		os.Exit(1)
	}

	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		common.Fatal("reading request from stdin", err)
	}

	workDirPath := path.Join(os.Args[1], request.Params.Repository)
	if err := os.Chdir(workDirPath); err != nil {
		common.Fatal("changing directory to "+workDirPath, err)
	}

	raw, err := ioutil.ReadFile(".git/merge-request.json")
	if err != nil {
		common.Fatal("unmarshalling merge request information", err)
	}

	var mr gitlab.MergeRequest
	json.Unmarshal(raw, &mr)

	api := gitlab.NewClient(common.GetDefaultClient(request.Source.Insecure), request.Source.PrivateToken)
	api.SetBaseURL(request.Source.GetBaseURL())

	if request.Params.Status != "" {
		message = message + fmt.Sprintf("Set Status: %s \n", request.Params.Status)
		state := gitlab.BuildState(gitlab.BuildStateValue(request.Params.Status))
		target := request.Source.GetTargetURL()
		name := request.Source.GetPipelineName()
		options := gitlab.SetCommitStatusOptions{
			Name:      &name,
			TargetURL: &target,
			State:     *state,
		}

		_, res, err := api.Commits.SetCommitStatus(mr.SourceProjectID, mr.SHA, &options)
		if res.StatusCode != 201 {
			body, _ := ioutil.ReadAll(res.Body)
			log.Fatalf("Set commit status failed: %d, response %s", res.StatusCode, string(body))
		}
		if err != nil {
			common.Fatal("Set commit status failed", err)
		}
	}

	commentBody := request.Params.Comment.GetContent(os.Args[1], request)
	if commentBody != "" {
		message = message + fmt.Sprintf("New comment: %s \n", commentBody)
		options := gitlab.CreateMergeRequestNoteOptions{
			Body: &commentBody,
		}
		_, res, err := api.Notes.CreateMergeRequestNote(mr.ProjectID, mr.IID, &options)
		if res.StatusCode != 201 {
			body, _ := ioutil.ReadAll(res.Body)
			log.Fatalf("Add merge request comment failed: %d, response %s", res.StatusCode, string(body))
		}
		if err != nil {
			common.Fatal("Add merge request comment failed", err)
		}
	}

	if request.Params.RemoveLabels != nil || request.Params.AddLabels != nil {
		// Refresh MR to get latest labels
		refreshedMR, res, err := api.MergeRequests.GetMergeRequest(mr.ProjectID, mr.IID, &gitlab.GetMergeRequestsOptions{})
		if res.StatusCode != 200 {
			body, _ := ioutil.ReadAll(res.Body)
			log.Fatalf("Failed to refresh MR: %d, response %s", res.StatusCode, string(body))
		}
		if err != nil {
			common.Fatal("Failed to refresh MR", err)
		}
		mr = *refreshedMR

		currentLabels := mr.Labels
		newLabels := mr.Labels

		message = message + fmt.Sprintf("Current Labels: %s \n", currentLabels)

		if request.Params.RemoveLabels != nil {
			for _, rmLabel := range request.Params.RemoveLabels {
				for idx, currentLabel := range currentLabels {
					if rmLabel == currentLabel {
						newLabels = append(newLabels[:idx], newLabels[idx+1:]...)
						break
					}
				}
			}
		}

		if request.Params.AddLabels != nil {
			for _, newLabel := range request.Params.AddLabels {
				if !Contains(newLabels, newLabel) {
					newLabels = append(newLabels, newLabel)
				}
			}
		}

		message = message + fmt.Sprintf("New Labels: %s \n", newLabels)

		options := gitlab.UpdateMergeRequestOptions{
			Labels: &newLabels,
		}
		_, res, err = api.MergeRequests.UpdateMergeRequest(mr.ProjectID, mr.IID, &options)
		if res.StatusCode != 200 {
			body, _ := ioutil.ReadAll(res.Body)
			log.Fatalf("Update merge request failed: %d, response %s", res.StatusCode, string(body))
		}
		if err != nil {
			common.Fatal("Update merge request failed", err)
		}
	}

	if request.Params.Action != "" && request.Params.Action == "merge" {
		removeSourceBranch := true
		mergeWhenPipelineSucceeds := true
		options := gitlab.AcceptMergeRequestOptions{
			ShouldRemoveSourceBranch:  &removeSourceBranch,
			MergeWhenPipelineSucceeds: &mergeWhenPipelineSucceeds,
		}
		_, res, err := api.MergeRequests.AcceptMergeRequest(mr.ProjectID, mr.IID, &options)
		if res.StatusCode != 200 {
			body, _ := ioutil.ReadAll(res.Body)
			log.Fatalf("Update merge request failed: %d, response %s", res.StatusCode, string(body))
		}
		if err != nil {
			common.Fatal("Update merge request failed", err)
		}
	}

	response := out.Response{
		Version: resource.Version{
			ID:        mr.IID,
			UpdatedAt: mr.UpdatedAt,
		},
		Metadata: buildMetadata(&mr, message),
	}

	json.NewEncoder(os.Stdout).Encode(response)
}

func buildMetadata(mr *gitlab.MergeRequest, message string) resource.Metadata {

	return []resource.MetadataField{
		{
			Name:  "id",
			Value: strconv.Itoa(mr.ID),
		},
		{
			Name:  "iid",
			Value: strconv.Itoa(mr.IID),
		},
		{
			Name:  "sha",
			Value: mr.SHA,
		},
		{
			Name:  "message",
			Value: message,
		},
		{
			Name:  "title",
			Value: mr.Title,
		},
		{
			Name:  "author",
			Value: mr.Author.Name,
		},
		{
			Name:  "source",
			Value: mr.SourceBranch,
		},
		{
			Name:  "target",
			Value: mr.TargetBranch,
		},
		{
			Name:  "url",
			Value: mr.WebURL,
		},
	}
}

func Contains(sl []string, v string) bool {
	for _, vv := range sl {
		if vv == v {
			return true
		}
	}
	return false
}
