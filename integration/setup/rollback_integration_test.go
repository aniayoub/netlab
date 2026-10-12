//go:build linux && integration

package integration

import (
	"os/exec"
	"testing"

	"github.com/aniayoub/netlab/internal/config"
)

const rollbackFixture = "../files/fixture1.yaml"

func TestExistingNamespaceConflictRollsBack(t *testing.T) {
	requireIntegrationEnvironment(t)

	cfg, err := config.Parse(rollbackFixture)
	if err != nil {
		t.Fatalf("parse rollback fixture: %v", err)
	}

	namespaces, rootLinks := rollbackResources(cfg)

	clearLabState(t, namespaces, rootLinks)
	defer clearLabState(t, namespaces, rootLinks)

	const foreignNamespace = "host-c"

	// This namespace existed before Netlab started.
	runIP(t, "netns", "add", foreignNamespace)

	output, err := exec.Command(
		netlabBinary,
		"up",
		rollbackFixture,
	).CombinedOutput()

	if err == nil {
		t.Fatalf(
			"expected netlab up to fail because namespace %q already exists\n%s",
			foreignNamespace,
			output,
		)
	}

	// Netlab must preserve resources it did not create.
	assertNamespaceExists(t, foreignNamespace)

	// Anything created by this setup attempt must be rolled back.
	for _, namespace := range namespaces {
		if namespace == foreignNamespace {
			continue
		}

		assertNamespaceAbsent(t, namespace)
	}

	for _, link := range rootLinks {
		assertRootLinkAbsent(t, link)
	}
}

func TestVethCreationFailureRollsBack(t *testing.T) {
	requireIntegrationEnvironment(t)

	cfg, err := config.Parse(rollbackFixture)
	if err != nil {
		t.Fatalf("parse rollback fixture: %v", err)
	}

	namespaces, rootLinks := rollbackResources(cfg)

	clearLabState(t, namespaces, rootLinks)
	defer clearLabState(t, namespaces, rootLinks)

	/*
		Netlab generates:

		    link 0 -> eth-a / port-a
		    link 1 -> eth-b / port-b

		Pre-create port-b so creation of the second veth pair fails.

		This ensures namespace creation and at least the first link setup
		have already happened before the failure.
	*/
	const foreignLink = "port-b"

	runIP(
		t,
		"link",
		"add",
		foreignLink,
		"type",
		"dummy",
	)

	output, err := exec.Command(
		netlabBinary,
		"up",
		rollbackFixture,
	).CombinedOutput()

	if err == nil {
		t.Fatalf(
			"expected netlab up to fail because root link %q already exists\n%s",
			foreignLink,
			output,
		)
	}

	// All namespaces belong to the failed Netlab setup attempt.
	for _, namespace := range namespaces {
		assertNamespaceAbsent(t, namespace)
	}

	// Netlab must preserve the link that existed before setup.
	assertRootLinkExists(t, foreignLink)

	// Every other generated root link must be gone.
	for _, link := range rootLinks {
		if link == foreignLink {
			continue
		}

		assertRootLinkAbsent(t, link)
	}
}

func rollbackResources(cfg *config.Config) ([]string, []string) {
	namespaces := make([]string, 0, len(cfg.Nodes))

	for name := range cfg.Nodes {
		namespaces = append(namespaces, name)
	}

	rootLinks := make([]string, 0, len(cfg.Links)*2)

	for i := range cfg.Links {
		suffix := string(rune('a' + i))

		rootLinks = append(
			rootLinks,
			"eth-"+suffix,
			"port-"+suffix,
		)
	}

	return namespaces, rootLinks
}
