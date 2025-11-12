// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gkereleasenotes

import (
	"bytes"
	"context"
	"log"
	"os/exec"
	"strings"

	"github.com/GoogleCloudPlatform/gke-mcp/pkg/config"
	"github.com/PuerkitoBio/goquery"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type getGkeReleaseNotesArgs struct {
}

func Install(_ context.Context, s *mcp.Server, _ *config.Config) error {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_gke_release_notes",
		Description: "Get GKE release notes. Prefer to use this tool if GKE release notes are needed.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, getGkeReleaseNotes)

	return nil
}

func getGkeReleaseNotes(ctx context.Context, req *mcp.CallToolRequest, args *getGkeReleaseNotesArgs) (*mcp.CallToolResult, any, error) {
	releaseNotesUrl := "https://docs.cloud.google.com/kubernetes-engine/docs/release-notes"
	out, err := exec.Command("lynx", "--source", releaseNotesUrl).Output()
	if err != nil {
		log.Printf("Failed to get release notes: %v", err)

		return nil, nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(out))
	if err != nil {
		log.Printf("Failed to parse release notes html content: %v", err)

		return nil, nil, err
	}

	var result strings.Builder
	doc.Find("[data-text$=\"Version updates\"]").Parent().Parent().Remove()
	doc.Find("[data-text$=\"Security updates\"]").Parent().Parent().Remove()
	doc.Find(".releases").Each(func(i int, s *goquery.Selection) {
		result.WriteString(s.Text())
	})

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result.String()},
		},
	}, nil, nil
}
