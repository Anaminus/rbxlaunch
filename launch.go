package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/anaminus/rbxauth"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

var AllUsers = filepath.Join(programFiles(), `Roblox\Versions`)
var CurrentUser = filepath.Join(localAppData(), `Roblox\Versions`)
var Executables = []string{"RobloxPlayerBeta.exe"}

func localAppData() string {
	lappdata := os.Getenv("LOCALAPPDATA")
	if _, err := os.Stat(lappdata); lappdata == "" || err != nil {
		userProfile := os.Getenv("USERPROFILE")
		lappdata = filepath.Join(userProfile, `AppData\Local`)
		if _, err := os.Stat(lappdata); lappdata == "" || err != nil {
			lappdata = filepath.Join(userProfile, `Local Settings\Application Data`)
		}
	}
	return lappdata
}

func programFiles() string {
	programFiles := `C:\Program Files (x86)`
	if _, err := os.Stat(programFiles); err != nil {
		programFiles = `C:\Program Files`
	}
	return programFiles
}

func findBuild(dirname string) string {
	if _, err := os.Stat(dirname); err != nil {
		return ""
	}

	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		return ""
	}
	for _, file := range files {
		if file.IsDir() {
			for _, exe := range Executables {
				exepath := filepath.Join(dirname, file.Name(), exe)
				if _, err := os.Stat(exepath); err == nil {
					return exepath
				}
			}
		}
	}
	return ""
}

func FindPlayer() (build, host string) {
	build = findBuild(AllUsers)
	if build == "" {
		build = findBuild(CurrentUser)
	}
	if build == "" {
		return
	}

	type AppSettings struct {
		BaseUrl string
	}

	b, err := ioutil.ReadFile(filepath.Join(filepath.Dir(build), "AppSettings.xml"))
	if err != nil {
		return
	}
	appSettings := AppSettings{}
	err = xml.Unmarshal(b, &appSettings)
	u, _ := url.Parse(appSettings.BaseUrl)
	if u == nil {
		return
	}
	host = u.Host
	return
}

type GameRequest struct {
	JobId                string `json:"jobId"`
	Status               int    `json:"status"`
	JoinScriptURL        string `json:"joinScriptUrl"`
	AuthenticationURL    string `json:"authenticationUrl"`
	AuthenticationTicket string `json:"authenticationTicket"`
}

func main() {
	var username string
	var placeID int

	// Parse flags.
	flag.StringVar(&username, "u", "", "Username to login with.")
	flag.IntVar(&placeID, "id", 0, "ID of place to join.")
	flag.Parse()

	if placeID == 0 {
		fmt.Fprintf(os.Stderr, "place ID required (-id)\n")
		return
	}

	// Find game client.
	player, host := FindPlayer()
	if player == "" {
		fmt.Fprintf(os.Stderr, "failed to locate game client. Make sure Roblox is installed.\n")
		return
	}

	// Log in with web client.
	client := &rbxauth.Client{}
	if username != "" {
		var password []byte
		var err error
		fmt.Print("Enter your password: ")
		password, err = terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read password: %s\n", err)
			return
		}
		if err := client.Login(host, username, password); err != nil {
			fmt.Fprintf(os.Stderr, "failed to log in: %s\n", err)
			return
		}
		copy(password, make([]byte, len(password)))
	}

	// Request game.
	gr := GameRequest{}
	{
		launchURL := &url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/game/placelauncher.ashx",
			RawQuery: url.Values{
				"request":       []string{"RequestGame"},
				"placeId":       []string{strconv.Itoa(placeID)},
				"isPartyLeader": []string{"false"},
				"gender":        []string{""},
			}.Encode(),
		}
		resp, err := client.Get(launchURL.String())
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to request game: %s\n", err)
			return
		}
		jd := json.NewDecoder(resp.Body)
		if err := jd.Decode(&gr); err != nil {
			fmt.Fprintf(os.Stderr, "failed to decode response: %s\n", err)
			resp.Body.Close()
			return
		}
		resp.Body.Close()

		// TODO: may be necessary to check job status:
		// ?request=CheckGameJobStatus&jobId=JoinPlace%3D[id]%3B
	}

	// Launch game client.
	{
		err := exec.Command(player,
			"--play",
			"-a", gr.AuthenticationURL,
			"-t", gr.AuthenticationTicket,
			"-j", gr.JoinScriptURL,
		).Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start game client: %s\n", err)
			return
		}
	}
}
