package wizard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/enolalab/alfred/internal/config"
	"k8s.io/client-go/tools/clientcmd"
)

func Run(cfg *config.Config) error {
	var provider string
	var apiKey string
	var useK8s bool
	var k8sContext string

	// Check if kubeconfig exists
	kubeconfigPath := getKubeconfigPath()
	hasKubeconfig := false
	if _, err := os.Stat(kubeconfigPath); err == nil {
		hasKubeconfig = true
		// Try to read current context
		if kConfig, err := clientcmd.LoadFromFile(kubeconfigPath); err == nil && kConfig.CurrentContext != "" {
			k8sContext = kConfig.CurrentContext
		}
	}

	// Workaround for huh.Append which might not exist or be easy to use with groups dynamically.
	// We'll just build the groups slice conditionally.
	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewNote().
				Title("Welcome to Alfred!").
				Description("It looks like this is your first run. Let's set up some basics."),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which LLM provider do you want to use?").
				Options(
					huh.NewOption("Anthropic (Recommended)", "anthropic"),
					huh.NewOption("OpenAI", "openai"),
					huh.NewOption("Gemini", "gemini"),
					huh.NewOption("OpenRouter", "openrouter"),
				).
				Value(&provider),
			huh.NewInput().
				Title("Enter your API Key").
				Description("This will be stored securely in ~/.alfred/config.yml").
				EchoMode(huh.EchoModePassword).
				Value(&apiKey).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("API key cannot be empty")
					}
					return nil
				}),
		),
	}

	if hasKubeconfig {
		prompt := "I found a kubeconfig file"
		if k8sContext != "" {
			prompt += fmt.Sprintf(" (context: %s)", k8sContext)
		}
		prompt += ".\nDo you want me to use it in Read-only mode?"

		groups = append(groups, huh.NewGroup(
			huh.NewConfirm().
				Title(prompt).
				Affirmative("Yes").
				Negative("No").
				Value(&useK8s),
		))
	}

	form := huh.NewForm(groups...)

	err := form.Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errors.New("setup aborted by user")
		}
		return err
	}

	fmt.Println("\nApplying configuration...")

	// Apply configurations
	cfg.LLM.Provider = provider
	cfg.LLM.APIKey = strings.TrimSpace(apiKey)

	if useK8s {
		cfg.Tools.Kubernetes.Enabled = true
		cfg.Tools.Kubernetes.Mode = "ex_cluster"
		cfg.Tools.Kubernetes.KubeconfigPath = kubeconfigPath
		if k8sContext != "" {
			cfg.Tools.Kubernetes.Context = k8sContext
		}
		fmt.Println("🛡️ Security Note: Kubernetes tools are configured for Read-only operations locally.")
	}

	// Save configuration
	if err := cfg.Save(""); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println("✅ Configuration saved! You can now start using Alfred.")
	fmt.Println("Try running: alfred chat \"Are there any pods failing?\"")

	return nil
}

func getKubeconfigPath() string {
	if k := os.Getenv("KUBECONFIG"); k != "" {
		return k
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube", "config")
}
