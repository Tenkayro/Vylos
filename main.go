package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"familycontrol/service"
)

//go:embed all:frontend/dist
var embeddedFrontend embed.FS

func main() {
	var (
		flagService          = flag.Bool("service", false, "Run as the background Windows Service (FamilyControlService)")
		flagInstallService   = flag.Bool("install-service", false, "Install and start the FamilyControl Windows Service")
		flagUninstallService = flag.Bool("uninstall-service", false, "Stop and remove the FamilyControl Windows Service (requires --password)")
		flagPassword         = flag.String("password", "", "Parent password for protected CLI operations")
		flagRecoverCode      = flag.String("recover", "", "Emergency parent recovery code to reset password")
		flagNewPassword      = flag.String("new-password", "", "New parent password when using --recover")
		flagHeadless         = flag.Bool("headless", false, "Run background agent without opening a desktop window")
		flagDataDir          = flag.String("data-dir", "", "Custom data directory for SQLite and keys")
		flagUIPort           = flag.Int("ui-port", 0, "Optional fixed local loopback port for the desktop UI (0 = automatic)")
	)
	flag.Parse()

	app, err := NewApp(*flagDataDir, true)
	if err != nil {
		log.Fatalf("Failed to initialize FamilyControl: %v", err)
	}
	defer app.Shutdown()

	if *flagInstallService {
		exePath, err := os.Executable()
		if err != nil {
			log.Fatalf("Cannot resolve executable path: %v", err)
		}
		if err := service.InstallWindowsService(exePath); err != nil {
			log.Fatalf("Service installation failed: %v", err)
		}
		fmt.Println("FamilyControlService installed and started successfully.")
		return
	}

	if *flagUninstallService {
		ok, err := app.AuthorizeUninstall(*flagPassword)
		if err != nil || !ok {
			log.Fatalf("Uninstall authorization denied: valid --password is required (%v)", err)
		}
		if err := service.UninstallWindowsService(); err != nil {
			log.Fatalf("Service removal failed: %v", err)
		}
		fmt.Println("FamilyControlService stopped and removed successfully.")
		return
	}

	if *flagRecoverCode != "" {
		if *flagNewPassword == "" {
			log.Fatalf("Please specify --new-password along with --recover")
		}
		newRecoveryKey, err := app.RecoverParentPassword(*flagRecoverCode, *flagNewPassword)
		if err != nil {
			log.Fatalf("Emergency recovery failed: %v", err)
		}
		fmt.Printf("Parent password reset successfully.\nNew Emergency Recovery Key (save this in a safe place): %s\n", newRecoveryKey)
		return
	}

	if *flagService || service.IsWindowsServiceSession() {
		if err := service.RunWindowsService(app.Agent()); err != nil {
			log.Fatalf("Windows service execution failed: %v", err)
		}
		return
	}

	// Desktop UI + System Tray Mode
	mux := http.NewServeMux()
	app.RegisterRPCBridge(mux)

	distFS, err := fs.Sub(embeddedFrontend, "frontend/dist")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(distFS)))
	}

	listenAddr := fmt.Sprintf("127.0.0.1:%d", *flagUIPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil && *flagUIPort != 0 {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		log.Fatalf("Failed to bind local UI loopback endpoint: %v", err)
	}
	uiURL := fmt.Sprintf("http://%s", ln.Addr().String())
	fmt.Printf("FamilyControl Desktop UI active at: %s\n", uiURL)
	fmt.Println("Press Ctrl+C to stop FamilyControl.")

	uiServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		_ = uiServer.Serve(ln)
	}()

	app.SetOnOpenUI(func() {
		openDesktopWindow(uiURL)
	})
	app.StartBackgroundRuntime(!*flagHeadless)

	if !*flagHeadless {
		openDesktopWindow(uiURL)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
}

// openDesktopWindow launches the FamilyControl interface in a native chromeless WebView2 / Edge app window.
func openDesktopWindow(url string) {
	if runtime.GOOS != "windows" {
		return
	}
	edgeCandidates := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	}
	for _, bin := range edgeCandidates {
		if _, err := os.Stat(bin); err == nil {
			cmd := exec.Command(bin,
				"--app="+url,
				"--window-size=1220,800",
				"--disable-extensions",
				"--disable-features=Translate",
			)
			_ = cmd.Start()
			return
		}
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
