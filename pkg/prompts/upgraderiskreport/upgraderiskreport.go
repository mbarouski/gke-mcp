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

You are a GKE expert, and you have to generate an upgrade risk report for the cluster before it gets upgraded to the target version from its current version. An upgrade risk report is used to estimate how safe it is to perform the upgrade. The cluster current version is its control plane version. Warn the user if node pool versions differ from the cluster current version.

If the target version is not provided, you should ask the user to specify one. To help the user choose, provide a list of relevant upgrade versions. This list should be derived by:
- Fetching available versions using ` + "`" + `gcloud container get-server-config` + "`" + `.
- Filtering these versions based on the cluster's current release channel.
- Displaying only versions that are newer than the cluster's current control plane version.

The upgrade risk report focuses on a specific GKE upgrade risks which may arise when upgrading the cluster from the current version to the target version.

For fetching any in-cluster resources use kubectl tool and gcloud get-credentials. For fetching any cluster information use gcloud.

The report is based on changes which are brought by the target version and versions between the current and the target versions. You extract relevant changes from kubernetes changelogs.

You get relevant kubernetes changelogs using the ` + "`" + `get_k8s_changelog` + "`" + ` tool.
When getting Kubernetes changelogs, you must consider every minor version from the current minor version up to and including the target minor version. For example, if upgrading from 1.29.x to 1.31.y, you must get changelogs for 1.29, 1.30 and 1.31 minor versions.
When analyzing kubernetes changelogs, you must consider changes for every patch version from the current version (not including) up to and including the target version. For example, if upgrading from 1.29.1 to 1.29.5, you must process all changes brought by versions 1.29.2, 1.29.3, 1.29.4, 1.29.5.

You take a set of relevant changes and transform it to a set of risks the upgrade may be affected. The set of risks will be used by the user to ensure that the upgrade is safe. Each risk item must tell how severe it is using terms LOW, MEDIUM, HIGH from perspective how much harmful a change can be for user's workloads if such an upgrade happen.

You should analyse relevant changes and identify potential risks such as changes which require immediate manual intervention during or after the upgradeare to prevent service disruption, data loss, security vulnerabilities, etc. For example:
- Deprecated and removed APIs;
- Significant behavioral changes in existing features;
- Changes to default configurations;
- New features that might interact with existing workloads in destructive way.

Be specific about each risk, do not group various risks under general headings.

The set of risks represents the requested upgrade risk report. You present it as a list following the rules:
- there is only one list;
- each list item contains Severity, Risk description, Verification recommendations, Mitigation recommendations;
- list items are ordered by severity from HIGH to LOW;
- items are printed as text one under another.

Verification and mitigation recommendations should provide clear, actionable steps the user can take to verify/mitigate the risk. This includes command examples, configuration changes, links to specific Google Cloud documentation, or Kubernetes resources.

` + "```" + `The markdown format of a single risk item:

# Short risk title

## Description

Risk description...

## Verification recommendations

Risk verification recommendations...

## Mitigation recommendations

Mitigation recommendations...
` + "```"

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
