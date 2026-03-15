package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ajesus37/hCTF2/internal/client"
	"github.com/ajesus37/hCTF2/internal/config"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to an hCTF2 server",
	RunE:  runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear saved credentials",
	RunE:  runLogout,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current server and auth status",
	RunE:  runStatus,
}

var (
	loginEmail    string
	loginPassword string
	loginServer   string
)

var (
	registerEmail    string
	registerName     string
	registerPassword string
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Create a new account",
	RunE:  runRegister,
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(registerCmd)
	loginCmd.Flags().StringVar(&loginEmail, "email", "", "Email address")
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "Password")
	loginCmd.Flags().StringVar(&loginServer, "server", "", "Server URL (e.g. http://localhost:8090)")
	registerCmd.Flags().StringVar(&registerEmail, "email", "", "Email address")
	registerCmd.Flags().StringVar(&registerName, "name", "", "Display name")
	registerCmd.Flags().StringVar(&registerPassword, "password", "", "Password")
}

func runRegister(_ *cobra.Command, _ []string) error {
	if term.IsTerminal(int(os.Stdin.Fd())) && registerEmail == "" {
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Email").Value(&registerEmail),
			huh.NewInput().Title("Name").Value(&registerName),
			huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&registerPassword),
		)).Run(); err != nil {
			return err
		}
	}
	if registerEmail == "" || registerPassword == "" {
		return fmt.Errorf("email and password are required")
	}

	serverURL := serverOverride
	if serverURL == "" {
		cfg, _ := config.Load()
		serverURL = cfg.Server
	}
	if serverURL == "" {
		return fmt.Errorf("server URL required (use --server or run 'hctf2 login' first)")
	}

	c := client.New(serverURL, "")
	if err := c.Register(registerEmail, registerName, registerPassword); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Registered as %s\n", registerEmail)
	}
	return nil
}

func runLogin(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()

	server := loginServer
	if server == "" && serverOverride != "" {
		server = serverOverride
	}
	if server == "" {
		server = cfg.Server
	}

	email := loginEmail
	password := loginPassword

	// Interactive prompts when running in a TTY and values missing
	if term.IsTerminal(int(os.Stdin.Fd())) && (email == "" || password == "") {
		var fields []huh.Field
		if loginServer == "" && server == "http://localhost:8090" {
			fields = append(fields, huh.NewInput().Title("Server URL").Value(&server))
		}
		if email == "" {
			fields = append(fields, huh.NewInput().Title("Email").Value(&email))
		}
		if password == "" {
			fields = append(fields, huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&password))
		}
		if len(fields) > 0 {
			if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
				return err
			}
		}
	}

	if email == "" || password == "" {
		return fmt.Errorf("--email and --password required")
	}

	c := client.New(server, "")
	lr, err := c.Login(email, password)
	if err != nil {
		return err
	}

	cfg.Server = server
	cfg.Token = lr.Token
	cfg.TokenExpires = jwtExpiry(lr.Token)
	if err := config.Save(cfg); err != nil {
		return err
	}

	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Logged in to %s\n", server)
	}
	return nil
}

func runLogout(_ *cobra.Command, _ []string) error {
	cfg, _ := config.Load()
	cfg.Token = ""
	cfg.TokenExpires = time.Time{}
	if err := config.Save(cfg); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintln(os.Stdout, "Logged out")
	}
	return nil
}

func runStatus(_ *cobra.Command, _ []string) error {
	cfg, _ := config.Load()
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(cfg)
	}
	fmt.Fprintf(os.Stdout, "Server:  %s\n", cfg.Server)
	if cfg.Token == "" {
		fmt.Fprintln(os.Stdout, "Auth:    not logged in")
	} else if time.Now().After(cfg.TokenExpires) {
		fmt.Fprintln(os.Stdout, "Auth:    session expired — run 'hctf2 login'")
	} else {
		user := jwtSubject(cfg.Token)
		fmt.Fprintf(os.Stdout, "Auth:    %s (expires %s)\n", user, cfg.TokenExpires.Format(time.RFC3339))
	}
	return nil
}

// decodeJWTPayload base64-decodes and unmarshals the payload of a JWT without verifying the signature.
func decodeJWTPayload(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(b, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// jwtExpiry extracts exp claim from a JWT without verifying signature.
func jwtExpiry(token string) time.Time {
	claims, err := decodeJWTPayload(token)
	if err != nil {
		return time.Now().Add(24 * time.Hour)
	}
	exp, _ := claims["exp"].(float64)
	if exp == 0 {
		return time.Now().Add(24 * time.Hour)
	}
	return time.Unix(int64(exp), 0)
}

// jwtSubject extracts the email claim from a JWT without verifying.
func jwtSubject(token string) string {
	claims, err := decodeJWTPayload(token)
	if err != nil {
		return "unknown"
	}
	if email, _ := claims["email"].(string); email != "" {
		return email
	}
	sub, _ := claims["sub"].(string)
	return sub
}
