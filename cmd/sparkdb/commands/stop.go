package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var stopHost string
var stopPort int
var stopUser string
var stopPass string
var stopAPIKey string

func init() {
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the SparkDB server gracefully",
		RunE:  runStop,
	}
	stopCmd.Flags().StringVar(&stopHost, "host", "localhost", "server host")
	stopCmd.Flags().IntVar(&stopPort, "port", 9600, "server port")
	stopCmd.Flags().StringVar(&stopUser, "user", "", "login username")
	stopCmd.Flags().StringVar(&stopPass, "pass", "", "login password")
	stopCmd.Flags().StringVar(&stopAPIKey, "api-key", "", "API key (alternative to user/pass)")
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	baseURL := fmt.Sprintf("http://%s:%d", stopHost, stopPort)
	client := &http.Client{}

	token := stopAPIKey
	if token == "" {
		if stopUser == "" || stopPass == "" {
			return fmt.Errorf("credentials required: use --user and --pass, or --api-key")
		}
		body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, stopUser, stopPass)
		resp, err := client.Post(baseURL+"/auth/login", "application/json", strings.NewReader(body))
		if err != nil {
			return fmt.Errorf("login: %w", err)
		}
		var res struct {
			Token string `json:"token"`
		}
		json.NewDecoder(resp.Body).Decode(&res)
		resp.Body.Close()
		token = res.Token
	}

	req, _ := http.NewRequest("POST", baseURL+"/shutdown", nil)
	if strings.HasPrefix(token, "vl_") {
		req.Header.Set("X-API-Key", token)
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	defer resp.Body.Close()

	var res struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if res.Error != "" {
		return fmt.Errorf("%s", res.Error)
	}
	fmt.Println(res.Message)
	return nil
}
