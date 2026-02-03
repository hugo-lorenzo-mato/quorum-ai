package issues

import "github.com/hugo-lorenzo-mato/quorum-ai/internal/service"

func (g *Generator) buildExpectedIssueFiles(consolidatedPath string, taskFiles []service.IssueTaskFile) []expectedIssueFile {
	expected := make([]expectedIssueFile, 0, len(taskFiles)+1)
	if consolidatedPath != "" {
		expected = append(expected, expectedIssueFile{
			FileName: mainIssueFilename,
			TaskID:   "main",
			IsMain:   true,
		})
	}

	for _, task := range taskFiles {
		t := task
		expected = append(expected, expectedIssueFile{
			FileName: issueFilenameForTask(task),
			TaskID:   task.ID,
			IsMain:   false,
			Task:     &t,
		})
	}

	return expected
}
