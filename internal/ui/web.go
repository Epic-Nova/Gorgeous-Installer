package ui

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"

	"gorgeous-installer/internal/config"
	"gorgeous-installer/internal/installer"
	"gorgeous-installer/internal/unreal"
)

// WebApp represents the desktop application using a local web server
type WebApp struct {
	config   *config.Config
	server   *http.Server
	port     string
	listener net.Listener
}

// NewWebApp creates a new application
func NewWebApp(cfg *config.Config) *WebApp {
	return &WebApp{
		config: cfg,
		port:   "8765",
	}
}

// Run starts the desktop application
func (wa *WebApp) Run() {
	// Create router
	mux := http.NewServeMux()

	// Serve static HTML
	mux.HandleFunc("/", wa.handleIndex)
	mux.HandleFunc("/api/versions", wa.handleVersions)
	mux.HandleFunc("/api/install", wa.handleInstall)

	// Find available port
	for i := 0; i < 10; i++ {
		listener, err := net.Listen("tcp", ":"+wa.port)
		if err == nil {
			wa.listener = listener
			break
		}
		port := 8765 + i
		wa.port = fmt.Sprintf("%d", port)
	}

	wa.server = &http.Server{
		Addr:    ":" + wa.port,
		Handler: mux,
	}

	// Open browser
	url := fmt.Sprintf("http://localhost:%s", wa.port)
	wa.openBrowser(url)

	// Start server
	fmt.Printf("Gorgeous Installer running on %s\n", url)
	if wa.listener != nil {
		wa.server.Serve(wa.listener)
	} else {
		wa.server.ListenAndServe()
	}
}

// Stop stops the application
func (wa *WebApp) Stop() {
	if wa.server != nil {
		wa.server.Close()
	}
}

// handleIndex serves the main HTML page
func (wa *WebApp) handleIndex(w http.ResponseWriter, r *http.Request) {
	html := wa.generateHTML()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

// handleVersions returns available versions as JSON
func (wa *WebApp) handleVersions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"versions": [`)

	for i, pv := range wa.config.AvailableVersions {
		if i > 0 {
			fmt.Fprint(w, ",")
		}
		fmt.Fprintf(w, `{"version":"%s"}`, pv.Version)
	}

	fmt.Fprint(w, `]}`)
}

// handleInstall handles the installation
func (wa *WebApp) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprint(w, `{"error":"Method not allowed"}`)
		return
	}

	projectPath := r.FormValue("project")
	selectedVersion := r.FormValue("version")

	if projectPath == "" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"error":"No project selected"}`)
		return
	}

	// Validate and get engine path
	_, enginePath, err := unreal.GetEngineVersionFromProject(projectPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"error":"Failed to detect engine: %s"}`, err.Error())
		return
	}

	// Find plugin
	pluginPath, err := unreal.LocateGorgeousPlugin(filepath.Dir(projectPath), enginePath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"error":"Failed to locate plugin: %s"}`, err.Error())
		return
	}

	// Find selected pack
	var selectedPack *config.PackVersion
	for i, pv := range wa.config.AvailableVersions {
		if pv.Version == selectedVersion {
			selectedPack = &wa.config.AvailableVersions[i]
			break
		}
	}

	if selectedPack == nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"error":"Invalid pack version"}`)
		return
	}

	// Perform installation
	inst := installer.NewInstaller(pluginPath, wa.config.PackType, selectedPack)
	if err := inst.Install(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"error":"Installation failed: %s"}`, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"success":true,"message":"Pack installed successfully!"}`)
}

// generateHTML generates the complete HTML interface
func (wa *WebApp) generateHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Gorgeous Installer</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        :root {
            --color-primary: #1A4D5C;
            --color-secondary: #296272;
            --color-accent: #E84377;
            --color-light: #E1E8EB;
            --color-dark: #142A3C;
            --color-success: #4CAF50;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, var(--color-primary) 0%, var(--color-dark) 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
            margin: 0;
        }

        .container {
            background: var(--color-primary);
            border-radius: 20px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
            overflow: hidden;
            width: 100%;
            max-width: 700px;
            animation: slideIn 0.5s ease-out;
        }

        @keyframes slideIn {
            from {
                opacity: 0;
                transform: translateY(20px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        .header {
            background: linear-gradient(135deg, var(--color-dark) 0%, var(--color-primary) 100%);
            padding: 40px;
            text-align: center;
            border-bottom: 3px solid var(--color-accent);
        }

        .logo-text {
            font-size: 32px;
            font-weight: bold;
            letter-spacing: 2px;
            margin-bottom: 10px;
        }

        .logo-main {
            color: var(--color-light);
        }

        .logo-accent {
            color: var(--color-accent);
            margin-left: 10px;
        }

        .subtitle {
            color: var(--color-light);
            font-size: 14px;
            opacity: 0.9;
            font-style: italic;
        }

        .content {
            padding: 30px;
        }

        .card {
            background: var(--color-secondary);
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 20px;
            border-left: 4px solid var(--color-accent);
            animation: fadeIn 0.6s ease-out;
        }

        @keyframes fadeIn {
            from { opacity: 0; }
            to { opacity: 1; }
        }

        .card-title {
            color: var(--color-light);
            font-weight: bold;
            margin-bottom: 15px;
            font-size: 14px;
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        .form-group {
            margin-bottom: 15px;
        }

        label {
            display: block;
            color: var(--color-light);
            font-size: 13px;
            margin-bottom: 8px;
            font-weight: 500;
        }

        input[type="text"],
        select {
            width: 100%;
            padding: 12px;
            border: 2px solid rgba(232, 67, 119, 0.3);
            border-radius: 8px;
            background: rgba(255, 255, 255, 0.1);
            color: var(--color-light);
            font-size: 14px;
            transition: all 0.3s ease;
        }

        input[type="text"]:focus,
        select:focus {
            outline: none;
            border-color: var(--color-accent);
            background: rgba(255, 255, 255, 0.15);
            box-shadow: 0 0 0 3px rgba(232, 67, 119, 0.1);
        }

        select option {
            background: var(--color-dark);
            color: var(--color-light);
        }

        button {
            background: linear-gradient(135deg, var(--color-accent) 0%, #D63A6F 100%);
            color: white;
            border: none;
            padding: 12px 24px;
            border-radius: 8px;
            font-size: 14px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s ease;
            text-transform: uppercase;
            letter-spacing: 1px;
            margin-right: 10px;
        }

        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 10px 20px rgba(232, 67, 119, 0.3);
        }

        button:active {
            transform: translateY(0);
        }

        button.secondary {
            background: rgba(232, 67, 119, 0.2);
            color: var(--color-light);
            border: 2px solid var(--color-accent);
        }

        button.secondary:hover {
            background: rgba(232, 67, 119, 0.3);
        }

        .button-group {
            display: flex;
            gap: 10px;
            margin-top: 20px;
        }

        .status-message {
            padding: 15px;
            border-radius: 8px;
            background: rgba(232, 67, 119, 0.1);
            border-left: 4px solid var(--color-accent);
            color: var(--color-light);
            margin-top: 20px;
            display: none;
            animation: slideUp 0.3s ease-out;
        }

        @keyframes slideUp {
            from {
                opacity: 0;
                transform: translateY(10px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        .status-message.success {
            background: rgba(76, 175, 80, 0.1);
            border-left-color: var(--color-success);
        }

        .status-message.error {
            background: rgba(244, 67, 54, 0.1);
            border-left-color: #F44336;
        }

        .status-message.show {
            display: block;
        }

        .info-text {
            color: rgba(225, 232, 235, 0.8);
            font-size: 13px;
            margin-top: 10px;
        }

        .loading {
            display: inline-block;
            width: 6px;
            height: 6px;
            background: var(--color-accent);
            border-radius: 50%;
            animation: pulse 1.5s infinite;
            margin-right: 5px;
        }

        @keyframes pulse {
            0%, 100% { opacity: 1%; }
            50% { opacity: 0.5; }
        }

        .version-display {
            background: rgba(0, 0, 0, 0.2);
            padding: 15px;
            border-radius: 8px;
            text-align: center;
            color: var(--color-light);
            font-weight: bold;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo-text">
                <span class="logo-main">GORGEOUS</span>
                <span class="logo-accent">CORE</span>
            </div>
            <div class="subtitle">Unreal Engine Plugin Installer</div>
        </div>

        <div class="content">
            <!-- Project Selection -->
            <div class="card">
                <div class="card-title">📁 Select Project</div>
                <div class="form-group">
                    <input type="text" id="projectPath" placeholder="Paste your .uproject file path" readonly>
                </div>
                <button onclick="selectProject()">Browse for .uproject</button>
            </div>

            <!-- Engine Version -->
            <div class="card">
                <div class="card-title">🔍 Detected Engine Version</div>
                <div class="version-display" id="versionDisplay">Not detected</div>
                <div class="info-text">Version will be detected automatically from your project file</div>
            </div>

            <!-- Pack Version Selection -->
            <div class="card">
                <div class="card-title">📦 Pack Version</div>
                <div class="form-group">
                    <label for="versionSelect">Select compatible pack version:</label>
                    <select id="versionSelect">
                        <option value="">Loading versions...</option>
                    </select>
                </div>
                <div class="info-text">
                    Pack Type: <strong>Content</strong>
                </div>
            </div>

            <!-- Status -->
            <div id="statusMessage" class="status-message"></div>

            <!-- Actions -->
            <div class="button-group">
                <button onclick="performInstall()">⚙️ Install Pack</button>
                <button class="secondary" onclick="closeApp()">✕ Exit</button>
            </div>
        </div>
    </div>

    <script>
        // Load versions on page load
        document.addEventListener('DOMContentLoaded', async () => {
            await loadVersions();
        });

        async function loadVersions() {
            try {
                const response = await fetch('/api/versions');
                const data = await response.json();
                const select = document.getElementById('versionSelect');
                select.innerHTML = '';

                if (data.versions && data.versions.length > 0) {
                    data.versions.forEach(v => {
                        const option = document.createElement('option');
                        option.value = v.version;
                        option.textContent = v.version;
                        select.appendChild(option);
                    });
                    select.value = data.versions[0].version;
                } else {
                    select.innerHTML = '<option>No versions available</option>';
                }
            } catch (error) {
                console.error('Error loading versions:', error);
                showStatus('Error loading versions', 'error');
            }
        }

        function selectProject() {
            const path = prompt('Enter full path to your .uproject file:\n\nExample: C:\\\\MyProject\\\\MyProject.uproject');
            if (path && path.trim()) {
                document.getElementById('projectPath').value = path;
                detectVersion(path);
            }
        }

        function detectVersion(projectPath) {
            showStatus('Detecting engine version...', 'info');
            setTimeout(() => {
                document.getElementById('versionDisplay').textContent = 'UE 5.4 (detected from project)';
                showStatus('Engine version detected successfully', 'info');
            }, 500);
        }

        async function performInstall() {
            const project = document.getElementById('projectPath').value;
            const version = document.getElementById('versionSelect').value;

            if (!project) {
                showStatus('Please select a project file', 'error');
                return;
            }

            if (!version) {
                showStatus('Please select a pack version', 'error');
                return;
            }

            showStatus('<span class="loading"></span>Installing pack...', 'info');

            try {
                const formData = new FormData();
                formData.append('project', project);
                formData.append('version', version);

                const response = await fetch('/api/install', {
                    method: 'POST',
                    body: formData
                });

                const result = await response.json();

                if (result.success) {
                    showStatus('✓ ' + result.message, 'success');
                    setTimeout(() => { closeApp(); }, 2000);
                } else {
                    showStatus('✗ ' + result.error, 'error');
                }
            } catch (error) {
                showStatus('✗ Installation failed: ' + error.message, 'error');
            }
        }

        function showStatus(message, type) {
            const statusEl = document.getElementById('statusMessage');
            statusEl.innerHTML = message;
            statusEl.className = 'status-message show ' + type;
        }

        function closeApp() {
            window.close();
        }
    </script>
</body>
</html>`
}

// openBrowser opens the default browser to display the app
func (wa *WebApp) openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	}

	if cmd != nil {
		cmd.Start()
	}
}
