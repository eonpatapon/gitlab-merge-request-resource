package out

import (
	"io/ioutil"
	"os"
	"path"
	"strings"

	resource "github.com/samcontesse/gitlab-merge-request-resource"
	"github.com/samcontesse/gitlab-merge-request-resource/common"
)

type Request struct {
	Source resource.Source `json:"source"`
	Params Params          `json:"params"`
}

type Response struct {
	Version  resource.Version  `json:"version"`
	Metadata resource.Metadata `json:"metadata"`
}

type Params struct {
	Repository   string   `json:"repository"`
	Status       string   `json:"status"`
	AddLabels    []string `json:"add_labels"`
	RemoveLabels []string `json:"remove_labels"`
	Comment      Comment  `json:"comment"`
	Action       string   `json:"action"`
}

type Comment struct {
	FilePath string `json:"file"`
	Text     string `json:"text"`
}

// Generate comment content
func (comment Comment) GetContent(basePath string, request Request) string {
	var (
		commentContent string
		fileContent    string
		buildTokens    = map[string]string{
			"${BUILD_URL}":           request.Source.GetTargetURL(),
			"${BUILD_ID}":            os.Getenv("BUILD_ID"),
			"${BUILD_NAME}":          os.Getenv("BUILD_NAME"),
			"${BUILD_JOB_NAME}":      os.Getenv("BUILD_JOB_NAME"),
			"${BUILD_PIPELINE_NAME}": request.Source.GetPipelineName(),
			"${ATC_EXTERNAL_URL}":    request.Source.GetCoucourseUrl(),
		}
	)
	if comment.FilePath != "" {
		filePath := path.Join(basePath, comment.FilePath)
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			common.Fatal("Can't read from "+filePath, err)
		} else {
			commentContent = string(content)
			fileContent = string(content)
		}
	}

	if comment.Text != "" {
		commentRaw := comment.Text
		commentContent = strings.Replace(commentRaw, "$FILE_CONTENT", fileContent, -1)
	}

	replaceTokens := func(sourceString string) string {
		for k, v := range buildTokens {
			sourceString = strings.Replace(sourceString, k, v, -1)
		}
		return sourceString
	}

	return replaceTokens(commentContent)
}
