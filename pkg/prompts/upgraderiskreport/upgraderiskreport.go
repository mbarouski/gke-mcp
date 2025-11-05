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

package upgraderiskreport

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/GoogleCloudPlatform/gke-mcp/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const gkeUpgradeRiskReportPromptTemplate = `
Cluster name: {{.clusterName}}
Cluster location: {{.clusterLocation}}
Target version: {{.targetVersion}}

You are a GKE expert, and you have to generate an upgrade risk report for the cluster before it gets upgraded to the target version from the current version. The current version is the cluster control plane version. Warn the user if node pool versions differ from the current version.

If the target version is not provided, ask the user providing them a list of available versions relying on "gcloud container get-server-config" and considering the cluster's current version and release channel.

Imagine, you have to upgrade the cluster, before that you need to estimate how safe it is to perform the upgrade to the target version. For that, you generate an upgrade risk report.

The report focuses on a specific GKE upgrade risks which may arise when upgrading the cluster from the current version to the target version.

For fetching any in-cluster resources use kubectl tool and gcloud get-credentials. For fetching any cluster information use gcloud.

The report is based on changes which are brought by the target version and versions between the current and the target versions. You extract relevant changes from GKE release notes and kubernetes changelog files.

You get GKE release notes from https://cloud.google.com/kubernetes-engine/docs/release-notes and use "lynx --dump [URL]" for fetching and converting HTML release notes to text.

You get relevant kubernetes changelog files using "curl [URL]" against URL corresponding to a minor version of interest. For example, https://raw.githubusercontent.com/kubernetes/kubernetes/master/CHANGELOG/CHANGELOG-1.31.md is a changelog file URL for kuberentes minor version 1.31.

When fetching Kubernetes changelogs, you download and analyze the changelog for every minor version from the current minor version up to and including the target minor version. For example, if upgrading from 1.29.x to 1.31.y, you must download and analyze CHANGELOG-1.29.md, CHANGELOG-1.30.md, and CHANGELOG-1.31.md changelog files.

When analyzing GKE release notes and/or kubernetes changelog files, you must consider changes for every patch version from the current version (not including) up to and including the target version. For example, if upgrading from 1.29.1-123 to 1.29.5-456, you must read CHANGELOG-1.29.md and process all changes brought by 1.29 in the version range (1.29.1; 1.29.5], i.e. 1.29.2, 1.29.3, 1.29.4, 1.29.5. Also you must read GKE release notes and process all changes included in the version range (1.29.1-123; 1.29.5-456], i.e. 1.29.1-234, 1.29.2-345, 1.29.3-400 and 1.29.5-456.

Always fetch the latest versions of these documents at the time the report is generated, as they can be updated.

Extracting changes from release notes and changelogs, you don't use grep, but use LLM capabilities. You use full content of these documents.

You take a set of relevant changes and transform it to a set of risks the upgrade may be affected. The set of risks will be used by the user to ensure that the upgrade is safe. Each risk item must tell how critical it is using terms LOW, MEDIUM, HIGH from perspective how much harmful a change can be for user's workloads if such an upgrade happen.

Your analysis of GKE release notes and kubernetes changelogs should identify potential risks such as:
- Deprecated and removed APIs
- Significant behavioral changes in existing features
- Changes to default configurations
- New features that might interact with existing workloads
- Security-related changes
- Changes likely to cause service disruption, data loss, security vulnerabilities, or require immediate manual intervention during or after the upgrade

The set of risks represents the requested upgrade risk report. You present it as a list following the rules:

- there is only one list;
- each list item contains Criticality, Risk description, Verification recommendations, Mitigation recommendations;
- list items are ordered by criticality from HIGH to LOW;
- items are printed as text one under another.

Verification and mitigation recommendations should provide clear, actionable steps the user can take to verify/mitigate the risk. This includes example commands, configuration changes, links to specific Google Cloud documentation, or Kubernetes resources.

The markdown format of a single risk item:

# Short risk title

## Description

Risk description...

## Verification recommendations

Risk verification recommendations...

## Mitigation recommendations

Mitigation recommendations...

`

var gkeUpgradeRiskReportTmpl = template.Must(template.New("gke-upgrade-risk-report").Parse(gkeUpgradeRiskReportPromptTemplate))

const (
	clusterNameArgName     = "cluster_name"
	clusterLocationArgName = "cluster_location"
	targetVersionArgName   = "target_version"
)

func Install(_ context.Context, s *mcp.Server, _ *config.Config) error {
	s.AddPrompt(&mcp.Prompt{
		Name:        "gke:upgraderiskreport",
		Description: "Generate GKE cluster upgrade risk report.",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        clusterNameArgName,
				Description: "A name of a GKE cluster user want to upgrade.",
				Required:    true,
			},
			{
				Name:        clusterLocationArgName,
				Description: "A location of a GKE cluster user want to upgrade.",
				Required:    true,
			},
			{
				Name:        targetVersionArgName,
				Description: "A version user want to upgrade their cluster to.",
				Required:    false,
			},
		},
	}, gkeUpgradeRiskReportHandler)

	return nil
}

// gkeUpgradeRiskReportHandler is the handler function for the /gke:upgraderiskreport prompt
func gkeUpgradeRiskReportHandler(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	clusterName := strings.TrimSpace(request.Params.Arguments[clusterNameArgName])
	if clusterName == "" {
		return nil, fmt.Errorf("argument '%s' cannot be empty", clusterNameArgName)
	}
	clusterLocation := strings.TrimSpace(request.Params.Arguments[clusterLocationArgName])
	if clusterLocation == "" {
		return nil, fmt.Errorf("argument '%s' cannot be empty", clusterLocationArgName)
	}
	targetVersion := strings.TrimSpace(request.Params.Arguments[targetVersionArgName])

	var buf bytes.Buffer
	if err := gkeUpgradeRiskReportTmpl.Execute(&buf, map[string]string{
		"clusterName":     clusterName,
		"clusterLocation": clusterLocation,
		"targetVersion":   targetVersion,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute prompt template: %w", err)
	}

	return &mcp.GetPromptResult{
		Description: "GKE Cluster Upgrade Risk Report Prompt",
		Messages: []*mcp.PromptMessage{
			{
				Content: &mcp.TextContent{
					Text: buf.String(),
				},
				Role: "user",
			},
		},
	}, nil
}
