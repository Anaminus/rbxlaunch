package main

import (
	"bufio"
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
	"sort"
	"strconv"
	"strings"
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

type State struct {
	Client *rbxauth.Client
	Player string
	Host   string
}

func (state *State) Message(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format, v...)
}

func (state *State) Login(username string) bool {
	var password []byte
	var err error
	state.Message("Enter your password: ")
	password, err = terminal.ReadPassword(int(syscall.Stdin))
	state.Message("\n")
	if err != nil {
		state.Message("failed to read password: %s\n", err)
		return false
	}
	if err := state.Client.Login(state.Host, username, password); err != nil {
		if err == rbxauth.ErrLoggedIn {
			state.Message("%s\n", err)
		} else {
			state.Message("failed to log in: %s\n", err)
		}
		return false
	}
	return true
}

func (state *State) Logout() {
	state.Client.Logout(state.Host)
}

func (state *State) Join(placeID int) {
	gr := GameRequest{}
	{
		launchURL := &url.URL{
			Scheme: "https",
			Host:   state.Host,
			Path:   "/game/placelauncher.ashx",
			RawQuery: url.Values{
				"request":       []string{"RequestGame"},
				"placeId":       []string{strconv.Itoa(placeID)},
				"isPartyLeader": []string{"false"},
				"gender":        []string{""},
			}.Encode(),
		}
		resp, err := state.Client.Get(launchURL.String())
		if err != nil {
			state.Message("failed to request game: %s\n", err)
			return
		}
		jd := json.NewDecoder(resp.Body)
		if err := jd.Decode(&gr); err != nil {
			state.Message("failed to decode response: %s\n", err)
			resp.Body.Close()
			return
		}
		resp.Body.Close()

		// TODO: may be necessary to check job status:
		// ?request=CheckGameJobStatus&jobId=JoinPlace%3D[id]%3B
	}

	// Launch game client.
	{
		err := exec.Command(state.Player,
			"--play",
			"-a", gr.AuthenticationURL,
			"-t", gr.AuthenticationTicket,
			"-j", gr.JoinScriptURL,
		).Start()
		if err != nil {
			state.Message("failed to start game client: %s\n", err)
			return
		}
	}
}

func (state *State) Help(cmds Commands, arg string) {
	if arg == "" {
		state.Message("Commands:\n")
		sorted := make([]string, len(cmds))
		{
			i := 0
			for name := range cmds {
				sorted[i] = name
				i++
			}
		}
		sort.Strings(sorted)

		for _, name := range sorted {
			cmd := cmds[name]
			state.Message("\t%s\t%s\n", name, cmd.Summ)
		}
		return
	}

	if arg == "help" {
		state.Message("help [command]\n\tDisplay all commands, or details for a specific command.\n")
		return
	}

	cmd, ok := cmds[arg]
	if !ok {
		state.Message("Unknown command %q\n", arg)
		return
	}

	if cmd.Args == "" {
		state.Message("%s\n\t%s\n", arg, cmd.Desc)
	} else {
		desc := strings.Replace(cmd.Desc, "\n", "\n\t", -1)
		state.Message("%s %s\n\t%s\n", arg, cmd.Args, desc)
	}
}

func InteractiveMode(state *State, cmds Commands) {
	input := bufio.NewReader(os.Stdin)
	for {
		state.Message("\nrbxlaunch>")
		line, _ := input.ReadString('\n')
		line = strings.TrimSpace(line)
		i := strings.Index(line, " ")
		var name, arg string
		if i == -1 {
			name = line
		} else {
			name, arg = line[:i], strings.TrimSpace(line[i+1:])
		}

		if name == "" {
			continue
		}

		if name == "help" {
			state.Help(cmds, arg)
			continue
		}

		cmd, ok := cmds[name]
		if !ok {
			state.Message("Unknown command %q. Type \"help\" for a list of commands.\n", name)
			continue
		}

		switch cmd.Func(state, arg) {
		case -1:
			// exit
			return
		case 0:
			// success
		case 1:
			// failure
		}
	}
}

type Commands map[string]Command

type Command struct {
	Args string
	Summ string
	Desc string
	Func func(state *State, arg string) int
}

func main() {
	var interactive bool
	var username string
	var placeID int

	// Parse flags.
	flag.BoolVar(&interactive, "i", false, "Force interactive mode.")
	flag.StringVar(&username, "u", "", "Username to login with.")
	flag.IntVar(&placeID, "id", 0, "ID of place to join.")
	flag.Parse()

	if len(os.Args) < 2 {
		interactive = true
	}

	state := &State{Client: &rbxauth.Client{}}

	// Find game client.
	state.Player, state.Host = FindPlayer()
	if state.Player == "" {
		state.Message("failed to locate game client. Make sure Roblox is installed.\n")
		return
	}

	if username != "" {
		if !state.Login(username) {
			return
		}
	}

	if interactive {
		InteractiveMode(state, Commands{
			"exit": Command{
				Args: "",
				Summ: "Terminate the program.",
				Desc: "Terminate the program.",
				Func: func(state *State, arg string) int {
					return -1
				},
			},
			"login": Command{
				Args: "USERNAME",
				Summ: "Login to a user account.",
				Desc: "Login to the account of the given username.\nYou will be prompted to enter the account's password.\nThe user session persists until the program terminates.",
				Func: func(state *State, username string) int {
					if username == "" {
						state.Message("username required.\n")
						return 1
					}
					state.Login(username)
					return 0
				},
			},
			"logout": Command{
				Args: "",
				Summ: "Logout the current user.",
				Desc: "Logout the current user.",
				Func: func(state *State, arg string) int {
					state.Logout()
					return 0
				},
			},
			"join": Command{
				Args: "ID",
				Summ: "Join a place.",
				// Desc: "Join place `ID`.\nIf `JOB` is given, the program attempts to join the associated server.\nIf you are not logged in, you will join as a guest.",
				Desc: "Join the place of the given ID.\nIf you have not logged in, then you will join as a guest.",
				Func: func(state *State, arg string) int {
					var placeID int
					fmt.Sscan(arg, &placeID)
					if placeID == 0 {
						state.Message("valid place ID required.")
						return 1
					}
					state.Join(placeID)
					return 0
				},
			},
		})
		return
	}

	if placeID == 0 {
		state.Message("place ID required (-id).\n")
		return
	}

	// Request game.
	state.Join(placeID)
}
