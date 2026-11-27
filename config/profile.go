package config

import (
	"SysInitPro/core"
	"encoding/json"
	"os"
)

const uiStateFile = "sysinit_ui_state.json"

type SetupProfile struct {
	ProfileName     string              `json:"profile_name"`
	TargetOS        string              `json:"target_os"`
	TargetRole      string              `json:"target_role"`
	CreateBackup    bool                `json:"create_backup"`
	InstallDrivers  bool                `json:"install_drivers"`
	AutoUpdateOS    bool                `json:"auto_update_os"`
	SecurityOnly    bool                `json:"security_only"`
	InstallRuntimes bool                `json:"install_runtimes"`
	OptimizeOptions []string            `json:"optimize_options"`
	SelectedMirror  string              `json:"selected_mirror"`
	SoftwareList    []core.SoftwareItem `json:"software_list"`
	DevExtra        core.DevEnvExtra    `json:"dev_extra"`
	NetExtra        core.NetworkExtra   `json:"net_extra"`
}

func ExportProfile(filename string, profile *SetupProfile) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func ImportProfile(filename string) (*SetupProfile, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var profile SetupProfile
	err = json.Unmarshal(data, &profile)
	return &profile, err
}

func SaveUIState(state map[string]string) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(uiStateFile, data, 0644)
}

func LoadUIState() map[string]string {
	state := make(map[string]string)
	data, err := os.ReadFile(uiStateFile)
	if err == nil {
		_ = json.Unmarshal(data, &state)
	}
	return state
}
