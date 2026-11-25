package files

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// Deleting a folder that was created after the index was built must not report an
// error when the delete itself succeeded.
func TestDeleteNewEmptyFolderNoError(t *testing.T) {
	for _, tc := range []struct {
		name         string
		siblingFiles bool // parent keeps other content after the delete
	}{
		{"parent keeps content", true},
		{"parent becomes empty", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			originalCache := settings.Config.Server.CacheDir
			settings.Config.Server.CacheDir = cacheDir
			t.Cleanup(func() { settings.Config.Server.CacheDir = originalCache })
			fileutils.SetFsPermissions(0o644, 0o755)
			if indexing.GetIndexDB() == nil {
				db, _, err := dbsql.NewIndexDB("test_delnew", "OFF", 1000, 32, false)
				if err != nil {
					t.Fatal(err)
				}
				indexing.SetIndexDBForTesting(db)
			}
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "scope", "parent"), 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.siblingFiles {
				if err := os.MkdirAll(filepath.Join(root, "scope", "parent", "Music"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "scope", "parent", "Music", "a.bin"), make([]byte, 50000), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			name := "test_delnew"
			indexing.Initialize(&settings.Source{Name: name, Path: root, Config: settings.SourceConfig{ResolvedRules: settings.ResolvedRulesConfig{
				// What a single root rule {folderPath: "/", ignoreSymlinks: true} resolves to.
				IgnoreAllSymlinks: true,
				FolderPaths:       map[string]settings.ConditionalRule{"/": {FolderPath: "/", IgnoreSymlinks: true}},
			}}}, false, false)
			idx := indexing.GetIndex(name)
			if idx == nil {
				t.Fatal("no index")
			}
			stop := startScannerStatusPoll(t, idx, 1)
			defer stop()
			waitForScannerReady(t, idx)
			defer func() {
				indexing.StopAllScanners()
				time.Sleep(50 * time.Millisecond)
				indexing.ClearTestIndices()
			}()

			if err := WriteDirectory("/scope", utils.FileOptions{Source: name, Path: "/scope/parent/new/", IsDir: true}); err != nil {
				t.Fatalf("WriteDirectory: %v", err)
			}
			err := DeleteFiles(name, "/scope", "/scope/parent/new/", true)
			if _, statErr := os.Stat(filepath.Join(root, "scope", "parent", "new")); !os.IsNotExist(statErr) {
				t.Fatalf("folder was not deleted on disk")
			}
			if err != nil {
				t.Errorf("DeleteFiles reported an error although the folder is gone: %v", err)
			}
		})
	}
}
