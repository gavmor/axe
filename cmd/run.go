package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/artifact"
	"github.com/jrswab/axe/internal/budget"
	"github.com/jrswab/axe/internal/config"
	"github.com/jrswab/axe/internal/mcpclient"
	"github.com/jrswab/axe/internal/memory"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/refusal"
	"github.com/jrswab/axe/internal/resolve"
	"github.com/jrswab/axe/internal/tool"
	"github.com/jrswab/axe/internal/wasmloader"
	"github.com/jrswab/axe/internal/xdg"
	"github.com/jrswab/axe/pkg/kernel"
	"github.com/jrswab/axe/pkg/protocol"
	"github.com/spf13/cobra"
)

// defaultUserMessage is sent when no stdin content is piped.
const defaultUserMessage = "Execute the task described in your instructions."

var runCmd = &cobra.Command{
	Use:   "run <agent>",
	Short: "Run an agent",
	Long: `Run an agent by loading its TOML configuration, resolving all runtime
context (working directory, file globs, skill, stdin), building a prompt,
calling the LLM provider, and printing the response.`,
	Args: exactArgs(1),
	RunE: runAgent,
}

func init() {
	runCmd.Flags().String("skill", "", "Override the agent's default skill path")
	runCmd.Flags().String("workdir", "", "Override the working directory")
	runCmd.Flags().String("agents-dir", "", "Additional agents directory to search before global config")
	runCmd.Flags().String("plugins-dir", "", "Directory containing .wasm plugins to load")
	runCmd.Flags().String("model", "", "Override the model (provider/model-name format)")
	runCmd.Flags().Int("timeout", 120, "Request timeout in seconds")
	runCmd.Flags().Bool("dry-run", false, "Show resolved context without calling the LLM")
	runCmd.Flags().BoolP("verbose", "v", false, "Print debug info to stderr")
	runCmd.Flags().Bool("json", false, "Wrap output in JSON with metadata")
	runCmd.Flags().StringP("prompt", "p", "", "Inline prompt to use as the user message")
	runCmd.Flags().Int("max-tokens", 0, "Maximum total tokens (input+output) for the entire run")
	runCmd.Flags().String("artifact-dir", "", "Override or set the artifact directory")
	runCmd.Flags().Bool("keep-artifacts", false, "Preserve auto-generated artifact directories")
	runCmd.Flags().Bool("stream", false, "Enable streaming responses from the LLM provider")
	rootCmd.AddCommand(runCmd)
}

func runAgent(cmd *cobra.Command, args []string) error {
	agentName := args[0]
	flagAgentsDir, _ := cmd.Flags().GetString("agents-dir")
	cwd, _ := os.Getwd()
	searchDirs := agent.BuildSearchDirs(flagAgentsDir, cwd)

	cfg, err := agent.Load(agentName, searchDirs)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	flagModel, _ := cmd.Flags().GetString("model")
	if flagModel != "" {
		cfg.Model = flagModel
	}

	provName, modelName, err := parseModel(cfg.Model)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	globalCfg, _ := config.Load()
	flagWorkdir, _ := cmd.Flags().GetString("workdir")
	workdir, _ := resolve.Workdir(flagWorkdir, cfg.Workdir)

	// Artifact system setup
	flagArtifactDir, _ := cmd.Flags().GetString("artifact-dir")
	keepArtifacts, _ := cmd.Flags().GetBool("keep-artifacts")
	var artifactDir string
	if flagArtifactDir != "" {
		artifactDir, _ = resolve.ExpandPath(flagArtifactDir)
	}
	artifactTracker := artifact.NewTracker()

	// Memory setup
	systemPrompt := cfg.SystemPrompt
	if cfg.Memory.Enabled {
		memoryPath, _ := memory.FilePath(agentName, cfg.Memory.Path)
		entries, _ := memory.LoadEntries(memoryPath, cfg.Memory.LastN)
		if entries != "" {
			systemPrompt += "\n\n---\n\n## Memory\n\n" + entries
		}
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	streamEnabled, _ := cmd.Flags().GetBool("stream")

	flagMaxTokens, _ := cmd.Flags().GetInt("max-tokens")
	tracker := budget.New(flagMaxTokens)

	promptFlag, _ := cmd.Flags().GetString("prompt")
	userMessage := promptFlag
	if userMessage == "" {
		userMessage = defaultUserMessage
	}

	apiKey := globalCfg.ResolveAPIKey(provName)
	baseURL := globalCfg.ResolveBaseURL(provName)
	prov, _ := provider.New(provName, apiKey, baseURL)

	var reqFormat *protocol.ResponseFormat
	if cfg.Format != nil {
		reqFormat = &protocol.ResponseFormat{}
		switch v := cfg.Format.(type) {
		case string:
			if v == "json" {
				reqFormat.Type = protocol.FormatJSON
			}
		case map[string]interface{}:
			reqFormat.Type = protocol.FormatSchema
			reqFormat.Schema = v
		}
	}

	req := &protocol.Request{
		Model:       modelName,
		System:      systemPrompt,
		Messages:    []protocol.Message{{Role: "user", Content: userMessage}},
		Temperature: cfg.Params.Temperature,
		MaxTokens:   cfg.Params.MaxTokens,
		Format:      reqFormat,
	}

	registry := tool.NewRegistry()
	tool.RegisterAll(registry)

	flagPluginsDir, _ := cmd.Flags().GetString("plugins-dir")
	var loader *wasmloader.Loader
	if flagPluginsDir != "" {
		loader, _ = wasmloader.New(cmd.Context())
	}

	start := time.Now()
	k := &kernel.Kernel{
		Config:          cfg,
		GlobalCfg:       globalCfg,
		AgentName:       agentName,
		Workdir:         workdir,
		AgentsDir:       flagAgentsDir,
		PluginsDir:      flagPluginsDir,
		Verbose:         verbose,
		JsonOutput:      jsonOutput,
		Stderr:          cmd.ErrOrStderr(),
		Stdout:          cmd.OutOrStdout(),
		ArtifactDir:     artifactDir,
		KeepArtifacts:   keepArtifacts,
		ArtifactTracker: artifactTracker,
		BudgetTracker:   tracker,
		WasmLoader:      loader,
		RegisterTools:   tool.RegisterAll,
	}

	resp, allToolCallDetails, totalInputTokens, totalOutputTokens, totalToolCalls, budgetExceeded, err := k.Run(cmd.Context(), prov, req, registry, nil, streamEnabled)
	if err != nil {
		return err
	}

	durationMs := time.Since(start).Milliseconds()

	if jsonOutput {
		envelope := map[string]interface{}{
			"model":             resp.Model,
			"content":           resp.Content,
			"input_tokens":      totalInputTokens,
			"output_tokens":     totalOutputTokens,
			"stop_reason":       resp.StopReason,
			"duration_ms":       durationMs,
			"tool_calls":        totalToolCalls,
			"tool_call_details": allToolCallDetails,
			"refused":           refusal.Detect(resp.Content),
		}
		data, _ := json.Marshal(envelope)
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		fmt.Fprint(cmd.OutOrStdout(), resp.Content)
	}

	if budgetExceeded {
		return &ExitError{Code: 4, Err: fmt.Errorf("budget exceeded")}
	}

	return nil
}

func parseModel(model string) (string, string, error) {
	idx := strings.Index(model, "/")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid format")
	}
	return model[:idx], model[idx+1:], nil
}
