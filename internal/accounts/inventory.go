package accounts

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type User struct {
	Name, Home, Shell string
	UID, GID          int
	Groups            []string
}

type Group struct {
	Name    string
	GID     int
	Members []string
}

type Inventory struct{ passwd, group string }

func NewInventory() *Inventory { return &Inventory{passwd: "/etc/passwd", group: "/etc/group"} }

func (i *Inventory) List() ([]User, []Group, error) { return readAccounts(i.passwd, i.group) }

func readAccounts(passwdPath, groupPath string) ([]User, []Group, error) {
	groups, err := readGroups(groupPath)
	if err != nil {
		return nil, nil, err
	}
	users, err := readUsers(passwdPath)
	if err != nil {
		return nil, nil, err
	}
	for index := range users {
		seen := map[string]bool{}
		for _, group := range groups {
			if group.GID == users[index].GID || contains(group.Members, users[index].Name) {
				if !seen[group.Name] {
					users[index].Groups = append(users[index].Groups, group.Name)
					seen[group.Name] = true
				}
			}
		}
		sort.Strings(users[index].Groups)
	}
	return users, groups, nil
}

func readUsers(path string) ([]User, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var result []User
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) != 7 {
			return nil, fmt.Errorf("invalid passwd entry")
		}
		uid, uidErr := strconv.Atoi(fields[2])
		gid, gidErr := strconv.Atoi(fields[3])
		if uidErr != nil || gidErr != nil {
			return nil, fmt.Errorf("invalid passwd identifier")
		}
		result = append(result, User{Name: fields[0], UID: uid, GID: gid, Home: fields[5], Shell: fields[6]})
	}
	return result, scanner.Err()
}

func readGroups(path string) ([]Group, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var result []Group
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) != 4 {
			return nil, fmt.Errorf("invalid group entry")
		}
		gid, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("invalid group identifier")
		}
		members := []string{}
		if fields[3] != "" {
			members = strings.Split(fields[3], ",")
		}
		result = append(result, Group{Name: fields[0], GID: gid, Members: members})
	}
	return result, scanner.Err()
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
