package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppControllerAndRPCBridge(t *testing.T) {
	tmpDir := t.TempDir()
	app, err := NewApp(tmpDir, false)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}
	defer app.Shutdown()

	state, err := app.GetBootstrapState()
	if err != nil {
		t.Fatalf("GetBootstrapState failed: %v", err)
	}
	if state.Config.SetupCompleted {
		t.Fatalf("expected fresh app to be unconfigured")
	}

	// Test RPC Bridge over HTTP
	mux := http.NewServeMux()
	app.RegisterRPCBridge(mux)

	setupBody, _ := json.Marshal(map[string]any{
		"familyName":     "RPC Test Family",
		"password":       "StrongParentPass123!",
		"parentContact":  "Call Mom",
		"firstChildName": "Maya",
	})
	req := httptest.NewRequest(http.MethodPost, "/rpc/SetupParentMode", bytes.NewReader(setupBody))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from SetupParentMode RPC, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify encrypted backup export and import
	backupBlob, err := app.ExportEncryptedBackup()
	if err != nil || backupBlob == "" {
		t.Fatalf("ExportEncryptedBackup failed: %v", err)
	}
	if err := app.ImportEncryptedBackup(backupBlob); err != nil {
		t.Fatalf("ImportEncryptedBackup failed: %v", err)
	}

	// Verify uninstall authorization requires valid parent password
	if ok, err := app.AuthorizeUninstall("WrongPassword"); err == nil || ok {
		t.Fatalf("expected wrong password to fail uninstall authorization")
	}
	if ok, err := app.AuthorizeUninstall("StrongParentPass123!"); err != nil || !ok {
		t.Fatalf("expected valid parent password to authorize uninstall: %v", err)
	}
}
