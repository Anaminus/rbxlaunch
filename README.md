# rbxlaunch

rbxlaunch is a command-line program that launches the Roblox game client
without requiring the Roblox website or a browser plugin.

If you want to join a game with your Roblox account, then you must enter your
username and password to log in. Use at your own discretion.

## Installation

1. [Install Go](https://golang.org/doc/install)
2. [Install Git](http://git-scm.com/downloads)
3. Using a shell with Git (such as Git Bash), run the following command:

```
go get -u github.com/anaminus/rbxlaunch
```

If you installed Go correctly, this will install rbxlaunch to `$GOPATH/bin`,
which will allow you run it directly from a shell.

Obviously, Roblox should also be installed.

## Usage

rbxlaunch is run from a shell.

```
rbxlaunch -id PLACEID [ -u USERNAME ]
```

The following options are available:

- `-id`: ID of place to join.
- `-u`: Username to log in with.

Specifying a username prompts you to enter the password of your Roblox user
account. This is required in order to join the game as a user.

If the username is not specified, then you will join the game as a guest.

## Examples

Login and enter game:

```
rbxlaunch -id 1818 -u Shedletsky
(prompts for password)
```

Enter game as a guest:

```
rbxlaunch -id 1818
```
