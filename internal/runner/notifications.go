package runner

import "strings"

func (r *Runner) notificationsEnabled() bool {
	if r == nil || r.settings == nil {
		return false
	}
	notify, found, err := r.settings.GetSetting("default_notify")
	if err != nil || !found {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(notify), "true")
}
