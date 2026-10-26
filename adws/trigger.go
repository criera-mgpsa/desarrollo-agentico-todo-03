package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type config struct {
	Repo            string `yaml:"repo"`
	Label           string `yaml:"label"`
	IntervalSeconds int    `yaml:"interval_seconds"`
	Prompt          string `yaml:"prompt"`
}

func readConfig(path string) (config, error) {
	var cfg config
	file, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("configuración YAML: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, fmt.Errorf("la configuración debe contener un solo documento YAML")
	}
	if strings.TrimSpace(cfg.Repo) == "" {
		return cfg, fmt.Errorf("configura repo con OWNER/REPO en %s", path)
	}
	if strings.TrimSpace(cfg.Label) == "" || cfg.IntervalSeconds < 1 {
		return cfg, fmt.Errorf("label no puede estar vacío e interval_seconds debe ser positivo")
	}
	if !strings.Contains(cfg.Prompt, "{{issue_url}}") {
		return cfg, fmt.Errorf("prompt debe contener {{issue_url}}")
	}
	return cfg, nil
}

func run(args []string) error {
	flags := flag.NewFlagSet("trigger-github", flag.ContinueOnError)
	path := flags.String("config", "config/adw.yml", "fichero YAML de configuración")
	once := flags.Bool("once", false, "procesa una sola consulta y termina")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("configura el repositorio y el prompt en %s", *path)
	}
	cfg, err := readConfig(*path)
	if err != nil {
		return err
	}
	for _, tool := range []string{"gh", "claude"} {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("no se encuentra %s en PATH", tool)
		}
	}

	for {
		query := exec.Command("gh", "issue", "list",
			"--repo", cfg.Repo, "--state", "open", "--label", cfg.Label,
			"--search", "sort:created-desc", "--limit", "100", "--json", "url", "--jq", ".[].url")
		query.Stderr = os.Stderr
		output, err := query.Output()
		if err != nil {
			return fmt.Errorf("consulta de issues: %w", err)
		}
		for _, issueURL := range strings.Split(string(output), "\n") {
			issueURL = strings.TrimSpace(issueURL)
			if issueURL == "" {
				continue
			}
			prompt := strings.ReplaceAll(cfg.Prompt, "{{issue_url}}", issueURL)
			fmt.Printf("\nLanzando %s\n", issueURL)
			agent := exec.Command("claude", "-p", prompt)
			agent.Stdout, agent.Stderr = os.Stdout, os.Stderr
			// Stdin queda cerrado: no hay conversación ni evaluación de shell.
			if err := agent.Run(); err != nil {
				return fmt.Errorf("Claude para %s: %w", issueURL, err)
			}
		}
		if *once {
			return nil
		}
		time.Sleep(time.Duration(cfg.IntervalSeconds) * time.Second)
	}
}

func exitCode(err error) int {
	var process *exec.ExitError
	if errors.As(err, &process) && process.ExitCode() > 0 {
		return process.ExitCode()
	}
	return 2
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCode(err))
	}
}
