package backup

import "github.com/alexcatdad/paw/internal/symlink"

func StatesToEntries(states []symlink.State) []SymlinkEntry {
	result := []SymlinkEntry{}
	for _, st := range states {
		if st.Status == symlink.StatusLinked || st.Status == symlink.StatusBackup {
			result = append(result, SymlinkEntry{Source: st.Source, Target: st.Target})
		}
	}
	return result
}
