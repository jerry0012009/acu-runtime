package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

func imageSkillSource() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "ACURuntime", "recommended-skills", "acu-image-tools")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	text, err := assets.ReadFile("web/recommended-image-skill.md")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, text, 0600); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	return dir, nil
}

// Use Magpie's existing skills library for client discovery and assignments.
// Do not edit agents' rules, built-in tools, approval lists or MCP permissions.
func (host *server) assignImageGuidance(ctx context.Context, agents []string) error {
	if len(agents) == 0 {
		return nil
	}
	source := host.imageSkillSource
	if source == nil {
		source = imageSkillSource
	}
	dir, err := source()
	if err != nil {
		return err
	}
	var lib struct {
		Skills []struct {
			Name   string   `json:"name"`
			Agents []string `json:"agents"`
			Source string   `json:"source"`
		} `json:"skills"`
	}
	if err := host.kernel.exchange(ctx, "GET", "/api/library", nil, &lib); err != nil {
		return err
	}
	for _, s := range lib.Skills {
		if s.Name != "acu-image-tools" {
			continue
		}
		if filepath.Clean(s.Source) != filepath.Clean(dir) {
			return errors.New("已有同名生图技能，未覆盖；请在工具目录检查 acu-image-tools")
		}
		ids := slices.Clone(s.Agents)
		for _, id := range agents {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		if slices.Equal(ids, s.Agents) {
			return nil
		}
		return host.writeImageGuidance(ctx, "/api/library/skills/agents", map[string]any{"name": s.Name, "agents": ids})
	}
	var probe struct {
		Candidates []struct{ Name, Path string } `json:"candidates"`
	}
	if err := host.kernel.exchange(ctx, "POST", "/api/library/skills/probe", map[string]string{"source": dir}, &probe); err != nil {
		return err
	}
	for _, candidate := range probe.Candidates {
		if candidate.Name == "acu-image-tools" {
			return host.writeImageGuidance(ctx, "/api/library/skills/install", map[string]any{"source": dir, "paths": []string{candidate.Path}, "agents": agents})
		}
	}
	return errors.New("内核未识别推荐生图技能")
}

func (host *server) writeImageGuidance(ctx context.Context, path string, input any) error {
	var result capabilityLibrary
	if err := host.kernel.exchange(ctx, "POST", path, input, &result); err != nil {
		return err
	}
	if len(result.Problems) > 0 || len(result.Result.Problems) > 0 {
		return errors.New("生图指引已保存，但部分应用分配失败，请重试")
	}
	return nil
}
