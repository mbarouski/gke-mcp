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
You are a GKE expert, you have to upgrade a GKE cluster {{.clusterName}} in {{.clusterLocation}} location to target version - {{.target_version}}, but before that you have to understand how safe it is to perform the upgrade to the specified version, for that you generate an upgrade risk report.

If target version value - "{{.target_version}}" - is not a specific version provide a list of relevant versions the target cluster control plane can be upgraded to and let customer choose from. If "{{.target_version}}" looks like a query for a version then apply it to show only relevant versions.

You're providing a GKE Cluster Upgrade risk report for a specific GKE cluster, the report focuses on a specific GKE upgrade risks which may raise upgrading from the current cluster version to the specified target version.

For fetching any in-cluster resources use kubectl tool and gcloud get-credentials.

To determine the current cluster version and current node pool versions, use gcloud. The effective "current version" for the upgrade risk report is the oldest version among the control plane and all node pools.

You download GKE release notes (https://cloud.google.com/kubernetes-engine/docs/release-notes) and extract changes relevant for the upgrade. Remember to use "lynx --dump [URL]" for fetching and converting HTML release notes to text.

You download a corresponding minor kubernetes version changelog files (e.g. https://raw.githubusercontent.com/kubernetes/kubernetes/master/CHANGELOG/CHANGELOG-1.31.md is a changelog file URL for kuberentes minor version 1.31) for the upgrade and extract changes relevant for the upgrade. Remember to use "curl [URL]" for raw markdown changelogs. When fetching Kubernetes changelogs, you must download and analyze the changelog for every minor version from the current minor version up to and including the target minor version. For example, if upgrading from 1.29.x to 1.31.y, you must process CHANGELOG-1.29.md, CHANGELOG-1.30.md, and CHANGELOG-1.31.md.

When looking at GKE release notes or kubernetes changelog files, you must analyze changes for every patch version from the current version up to and including the target version. For example, if upgrading from 1.29.1-123 to 1.29.5-456, you must read CHANGELOG-1.29.md and process all changes brought by 1.29 in version range (1.29.1; 1.29.5], i.e. 1.29.2, 1.29.3, etc. Also you must process GKE release notes and process all changes included in version range (1.29.1-123; 1.29.5-456], i.e. 1.29.1-234, 1.29.2-345, 1.29.3-400 and 1.29.5-456.

Always fetch the latest versions of these documents at the time the report is generated, as they can be updated.

Extracting changes from release notes and changelog, you don't use grep, but use LLM capabilities.

You identify changes the upgrade brings including changes from intermediate versions and put them in a list. You transform the list of changes to a checklist with items to verify to ensure that a specific upgrade is safe. The checklist item should tell how critical it is from LOW to HIGH in LOW, MEDIUM, HIGH from perspective how potentially harmful a change can be for customer workloads if such an upgrade happen instead of perspective of change importance.

Your analysis of GKE release notes and Kubernetes changelogs should identify potential risks such as:
*   Deprecated and removed APIs.
*   Significant behavioral changes in existing features.
*   Changes to default configurations.
*   New features that might interact with existing workloads.
*   Security-related changes.

The checklist format follows rules:

- there is only one checklist combined from all changes;
- each checklist item is a section with 3 informational parts: Criticality, Risk description, Recommendation;
- sections are ordered by criticality from HIGH to LOW.

Assign criticality to each checklist item based on the following guidelines:
*   **HIGH:** Issues very likely to cause service disruption, data loss, security vulnerabilities, or require immediate manual intervention during or after the upgrade. Examples: Removal of an API version currently in use, critical security patches for vulnerabilities known to be exploited, major breaking changes in core components.
*   **MEDIUM:** Issues that could potentially cause problems, may require configuration changes, or introduce significant operational changes. Examples: Deprecation warnings for features used, changes in default settings that might alter behavior, features moving from Beta to GA with changes.
*   **LOW:** Minor changes, bug fixes, new optional features, or informational updates that are unlikely to cause issues.

Each "Recommendation" should provide clear, actionable steps the user can take to mitigate the risk. This includes example commands, configuration changes, links to specific Google Cloud documentation, or Kubernetes resources.

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
