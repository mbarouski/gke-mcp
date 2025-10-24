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
You are a GKE expert, you have to upgrade a GKE cluster {{.clusterName}} in {{.clusterLocation}} location to {{.target_version}} version, but before that you have to understand how safe it is to perform the upgrade to the specified version, for that you generate an upgrade risk report.

You're providing a GKE Cluster Upgrade risk report for a specific GKE cluster, the report focuses on a specific GKE upgrade risks which may raise upgrading from the current cluster version to the specified target version.

For fetching any in-cluster resources use kubectl tool and gcloud get-credentials.

The current version is a lowest version among cluster control plane version and versions on cluster node pools.

You download GKE release notes (https://cloud.google.com/kubernetes-engine/docs/release-notes) and extract changes relevant for the upgrade. To download GKE release notes content, you use command line tool - lynx. You remember that release notes can be updated and need to be loaded again on each report generating.

You download a corresponding minor kubernetes version changelog files (e.g. https://raw.githubusercontent.com/kubernetes/kubernetes/master/CHANGELOG/CHANGELOG-1.31.md is a changelog file URL for kuberentes minor version 1.31) for the upgrade and extract changes relevant for the upgrade. To download kubernetes changelog file, you can use curl or lynx tools. You remember that a changelog file can be updated and need to be loaded again on each report generating.

Extracting changes from release notes and changelog, you don't use grep, but use LLM capabilities.

You identify changes the upgrade brings including changes from intermediate versions and put them in a list. You transform the list of changes to a checklist with items to verify to ensure that a specific upgrade is safe. The checklist item should tell how critical it is from LOW to HIGH in LOW, MEDIUM, HIGH.

The checklist format follows rules:

- there is only one checklist combined from all changes;
- each checklist item is a section with 3 informational parts: Criticality, Risk description, Recommendation;
- sections are ordered by criticality from HIGH to LOW.

An example of a checklist item:

` + "```" + `
HIGH: Potential for Network File System (NFS) volume mount failures

  * Criticality: HIGH
  * Risk description: In GKE versions 1.32.4-gke.1029000 and later, MountVolume calls for Network File System (NFS) volumes might fail with the error: mount.nfs: rpc.statd is not running but is required for remote locking. This can occur if a Pod mounting an NFS volume runs on the same node as an NFS server Pod, and the NFS server Pod starts before the client Pod attempts to mount the volume.
  * Recommendation: Before upgrading, deploy the recommended DaemonSet (https://cloud.google.com/kubernetes-engine/docs/release-notes#october_14_2025_2) on all nodes where you mount NFS volumes to ensure that the required services start correctly.
` + "```\n"

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
				Required:    true,
			},
		},
	}, gkeUpgradeRiskReportHandler)

	return nil
}

// gkeUpgradeRiskReportHandler is the handler function for the /gke:upgraderiskreport prompt
func gkeUpgradeRiskReportHandler(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	clusterName := request.Params.Arguments[clusterNameArgName]
	if strings.TrimSpace(clusterName) == "" {
		return nil, fmt.Errorf("argument '%s' cannot be empty", clusterNameArgName)
	}
	clusterLocation := request.Params.Arguments[clusterLocationArgName]
	if strings.TrimSpace(clusterLocation) == "" {
		return nil, fmt.Errorf("argument '%s' cannot be empty", clusterLocationArgName)
	}
	targetVersion := request.Params.Arguments[targetVersionArgName]
	if strings.TrimSpace(targetVersion) == "" {
		return nil, fmt.Errorf("argument '%s' cannot be empty", targetVersionArgName)
	}

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
