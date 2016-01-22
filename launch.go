package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/anaminus/rbxweb"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	path "path/filepath"
	"strconv"
)

var AllUsers = path.Join(programFiles(), `Roblox\Versions`)
var CurrentUser = path.Join(localAppData(), `Roblox\Versions`)
var Executables = []string{"RobloxPlayerBeta.exe"}

func localAppData() string {
	lappdata := os.Getenv("LOCALAPPDATA")
	if _, err := os.Stat(lappdata); lappdata == "" || err != nil {
		userProfile := os.Getenv("USERPROFILE")
		lappdata = path.Join(userProfile, `AppData\Local`)
		if _, err := os.Stat(lappdata); lappdata == "" || err != nil {
			lappdata = path.Join(userProfile, `Local Settings\Application Data`)
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
				exepath := path.Join(dirname, file.Name(), exe)
				if _, err := os.Stat(exepath); err == nil {
					return exepath
				}
			}
		}
	}
	return ""
}

func FindPlayer() string {
	build := findBuild(AllUsers)
	if build == "" {
		return findBuild(CurrentUser)
	}
	return build
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
	var password string
	var placeID int

	// Parse flags
	flag.StringVar(&username, "u", "", "Username to login with.")
	flag.StringVar(&password, "p", "", "Password to login with.")
	flag.IntVar(&placeID, "id", 0, "ID of place to join.")
	flag.Parse()

	if placeID == 0 {
		fmt.Fprintf(os.Stderr, "Place ID required (-id)\n")
		return
	}

	// Find game client
	player := FindPlayer()
	if player == "" {
		fmt.Fprintf(os.Stderr, "Failed to locate game client. Make sure Roblox is installed.\n")
		return
	}

	// Log in with web client
	client := rbxweb.NewClient()
	if username != "" && password != "" {
		if err := client.Login(username, password); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to log in: %s\n", err)
			return
		}
	}

	// Request game
	gr := GameRequest{}
	{
		resp, err := client.Get(client.GetSecureURL("assetgame", "/game/placelauncher.ashx", url.Values{
			"request":       []string{"RequestGame"},
			"placeId":       []string{strconv.Itoa(placeID)},
			"isPartyLeader": []string{"false"},
			"gender":        []string{""},
		}))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to request game: %s\n", err)
			return
		}
		jd := json.NewDecoder(resp.Body)
		if err := jd.Decode(&gr); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to decode response: %s\n", err)
			resp.Body.Close()
			return
		}
		resp.Body.Close()

		// TODO: may be necessary to check job status:
		// ?request=CheckGameJobStatus&jobId=JoinPlace%3D[id]%3B
	}

	// Launch game client
	{
		err := exec.Command(player,
			"--play",
			"-a", gr.AuthenticationURL,
			"-t", gr.AuthenticationTicket,
			"-j", gr.JoinScriptURL,
		).Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start game client: %s\n", err)
			return
		}
	}
}
