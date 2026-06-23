package mcp

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/marstid/skillpack/internal/skill"
)

// registerResources installs:
//   - a resource template `skill://{name}/{path}` for reading any bundled
//     file within a skill (scripts/, references/, assets/, or SKILL.md);
//   - a static resource per skill at `skill://<name>/SKILL.md` so the raw file
//     (including frontmatter) shows up in resources/list for clients that
//     prefer full-file reads over the activate_skill tool.
func registerResources(srv *mcp.Server, store *skill.Store) {
	srv.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "Skill resource",
		Description: "A bundled file within an Agent Skill (scripts/, references/, assets/, or SKILL.md). Replace {name} with the skill name and {path} with a file path relative to the skill directory (may contain slashes, e.g. references/rules.md).",
		URITemplate: "skill://{name}/{+path}",
	}, skillResourceHandler(store))

	for _, e := range store.List() {
		uri := fmt.Sprintf("skill://%s/SKILL.md", e.Name)
		// Capture name for the closure.
		name := e.Name
		srv.AddResource(&mcp.Resource{
			Name:        fmt.Sprintf("%s/SKILL.md", name),
			Description: fmt.Sprintf("Raw SKILL.md (with frontmatter) for the %q skill.", name),
			MIMEType:    "text/markdown",
			URI:         uri,
		}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			data, mime, err := store.ReadResource(name, "SKILL.md")
			if err != nil {
				return nil, mcp.ResourceNotFoundError(uri)
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{URI: uri, MIMEType: mime, Text: string(data)},
				},
			}, nil
		})
	}
}

// skillResourceHandler parses `skill://<name>/<path>` URIs and reads the
// bundled file from the skill store. Path traversal is rejected inside
// Store.ReadResource.
func skillResourceHandler(store *skill.Store) mcp.ResourceHandler {
	return func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		name, rel, err := parseSkillURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		data, mime, err := store.ReadResource(name, rel)
		if err != nil {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}
		// ResourceContents.Text is for text; for binary we'd use Blob, but the
		// agentskills.io bundled resources are all text-ish (md, scripts, refs).
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: req.Params.URI, MIMEType: mime, Text: string(data)},
			},
		}, nil
	}
}

// parseSkillURI splits a `skill://<name>/<path>` URI into the skill name and
// the file path relative to the skill directory. The {path} segment may
// contain slashes (e.g. references/rules.md).
func parseSkillURI(uri string) (name, relpath string, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("invalid skill URI %q: %w", uri, err)
	}
	if u.Scheme != "skill" {
		return "", "", fmt.Errorf("unsupported scheme %q in %q", u.Scheme, uri)
	}
	// url.Parse puts "name/path" into u.Opaque for "skill://name/path..."? No:
	// with a scheme + "://", u.Host is the first segment. For "skill://a/b/c",
	// Host="a" and Path="/b/c".
	host := u.Host
	pathPart := strings.TrimPrefix(u.Path, "/")
	if u.Opaque != "" {
		// Handles "skill:name/path" form; shouldn't occur but be defensive.
		host, pathPart, _ = strings.Cut(u.Opaque, "/")
	}
	if host == "" {
		return "", "", fmt.Errorf("skill URI %q is missing the skill name", uri)
	}
	if pathPart == "" {
		return "", "", fmt.Errorf("skill URI %q is missing the resource path", uri)
	}
	return host, pathPart, nil
}
