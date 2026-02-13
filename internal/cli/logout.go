package cli

import "fmt"

func Logout() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.Token = ""
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Println("Logged out")
	return nil
}
