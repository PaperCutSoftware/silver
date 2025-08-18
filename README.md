

# **Silver**

- *Only the best is dished up with Silver Service*

Silver is a robust, light-weight, cross-platform **service wrapper** for background applications. 

Silver takes a standard command-line program \- like a simple HTTP-based app or other background process \- and turns it into a resilient, auto-updating background service.  It's 100% cross platform and works across Windows, macOS, and Linux. It's designed to handle the operational realities of running software, like crashes, health monitoring, logging, cron-like scheduling and updates, so you can focus on your application's core logic.

Typical usages might include wrapping a single Java web application, hosting a set of Go microservices, or resiliently running a native "task tray" application on startup.

Silver is battle-tested and has been successfully used by [PaperCut Software](https://www.papercut.com/) to help manage server and desktop components for millions of laptops and servers for almost a decade.

---

## **Features**

Silver is packed with features to make your application robust and easy to manage in production environments.

* **Cross-Platform Service Management**: Runs your app as a native service (Windows Service, macOS LaunchAgent, Linux systemd/init) using a single, consistent interface.  
* **Process Resilience**: Automatically restarts your application services if they crash, with configurable limits (`MaxCrashCountPerHour`) and restart delays (`RestartDelaySecs`) to prevent rapid-restart CPU cycles.  
* **Health Monitoring**: Actively monitors your application service's health via HTTP(S) pings, TCP connection checks, TCP echo checks, or by watching for file changes. It automatically detects crashes, live-lock and dead-lock situations, and restarts the service on failure.  
* **Secure Auto-Updates**: A built-in `updater` binary fetches updates from a URL, supporting:  
  * Cryptographically signed update manifests (Ed25519) for security.  
  * Update package checksum validation (SHA256/SHA1).  
  * Post-update file operations (copy, move, exec, etc.).  
  * Post-install checks and operations to ensure an install is valid before the final atomic "move" to live.  
  * Update channels (e.g., `stable`, `beta`) for phased rollouts.  
* **Flexible Task Execution**:  
  * **Startup Tasks**: Run one-off tasks when the service starts, either synchronously or asynchronously.  
  * **Scheduled Tasks**: Run recurring tasks using powerful cron syntax.  
  * **Ad-Hoc Commands**: Expose custom command-line commands in a consistent way that can be triggered from the command line.  
* **Simple Configuration**: All behaviour is controlled by a single, comprehensive JSON configuration file.  
* **Logging**:  Built-in centralised logging for both Silver and your application's output.  Log buffing, flushing and rotation is automatically handled.  
* **System Integration**:  
  * Automatic service installation using native OS hooks.  
  * Automatic discovery of OS system's HTTP proxy settings.  
  * Support for running services under a specific user account (e.g. leverage least privilege).

---

## **How It Works**

Silver consists of two primary binaries that you build and deploy alongside your application:

1. `service`: The main service wrapper. It reads its configuration, manages your application's lifecycle, monitors its health, and runs tasks.  
2. `updater`: A standalone utility that handles the auto-update process. It's typically invoked as a `ScheduledTasks` or `StartupTask` by the `service` binary.

All configuration is defined in a JSON file, typically named `<service-name>.conf`, which lives in the same directory as the `service` executable.

---

## **Configuration (`<service-name>.conf`)**

The configuration file is the heart of Silver. Here is a comprehensive example with comments explaining each section.  

Note: While the example below uses comments for explanation, standard JSON does not support comments. They **must** be removed before use.

```
{
    // Basic information about your service, used for installation.
    "ServiceDescription": {
        "Name": "MyCoolApp",
        "DisplayName": "My Cool Application Server",
        "Description": "Does cool things in the background."
    },

    // Global settings for the Silver service wrapper itself.
    "ServiceConfig": {
        // Log file for Silver's own output, AND your Services.
        "LogFile": "${ServiceRoot}/${ServiceName}.log",
        "LogFileMaxSizeMb": 50,
        "LogFileMaxBackupFiles": 5,

        // File to store the current main service PID.
        "PidFile": "${ServiceRoot}/${ServiceName}.pid",

        // Optional files used to signal the service.
        "StopFile": ".stop",     // Creating this file signals a graceful shutdown.
        "ReloadFile": ".reload", // Creating this file triggers a full restart and config reload.

        // Run the service as a specific user (on macOS/Linux).
        "UserName": ""
    },

    // Include config from sub components. Include files may be Glob patterns.
    // Useful for separating concerns or managing config that is shipped with updated versioned components.
    "Include": [
        "${ServiceRoot}/components/v*/component.conf"
    ],

    // Environment variables to be set for all child processes.
    "EnvironmentVars": {
        "MY_APP_MODE": "production",
        "DATABASE_URL": "user@tcp(127.0.0.1:3306)/dbname"
    },

    // The main, long-running applications to be managed by Silver.
    "Services": [
        {
            "Path": "${ServiceRoot}/bin/my-app-server.exe",
            "Args": ["--port", "8080"],
            
            // Resilience settings
            "GracefulShutdownTimeoutSecs": 10, // Time to wait for clean exit before killing.
            "RestartDelaySecs": 5,             // Wait 5s before restarting after a crash.
            "MaxCrashCountPerHour": 10,        // Stop restarting if it crashes >10 times in an hour.

            // Health monitoring settings
            "MonitorPing": {
                "URL": "http://localhost:8080/health", // The URL to ping.
                "IntervalSecs": 30,                    // Ping every 30s.
                "TimeoutSecs": 5,                      // Ping times out after 5s.
                "StartupDelaySecs": 60,                // Wait 60s after service start before monitoring.
                "RestartOnFailureCount": 3             // Restart the service after 3 consecutive failures.
            }
        },
        {
              // Another service started with the latest installed version selected using a Glob pattern.
"Path": "${ServiceRoot}/v*/my-versioned-microservice.exe",
        }
    ],

    // One-off tasks to run when the service starts.
    "StartupTasks": [
        {
            "Path": "${ServiceRoot}/v*/db-migrate-check.exe",
            "Args": ["up"],
            "Async": false, // `false` means Silver waits for this to complete before starting Services.
            "TimeoutSecs": 300
        },
        {
            "Path": "${ServiceRoot}/updater.exe",
            "Args": ["https://updates.example.com/mycoolapp/manifest.json", "--public-key=YOUR_BASE64_PUBLIC_KEY"],
            "Async": true, // `true` means this runs in the background.
            "StartupDelaySecs": 60,
            "StartupRandomDelaySecs": 300 // Add a random delay to spread out update checks.
        }
    ],

    // Tasks to run on a recurring schedule.
    "ScheduledTasks": [
        {
            "Schedule": "0 0 3 * * *", // Cron syntax: 3 AM every day.
            "Path": "${ServiceRoot}/bin/cleanup-tool.exe",
            "Args": ["--older-than", "30d"],
            "TimeoutSecs": 3600 // Kill if it runs for more than 1 hour.
        },
        // Do update check daily as well as startup
        {
            "Schedule": "0 0 13 * * *", // 1 PM every day
            "Path": "${ServiceRoot}/updater.exe",
            "Args": ["https://updates.example.com/mycoolapp/manifest.json", "--public-key=YOUR_BASE64_PUBLIC_KEY"],
            "StartupRandomDelaySecs": 3600,
            "TimeoutSecs": 3600
        }

    ],

    // Ad-hoc commands you can run via the CLI: `service.exe command <name>`
    "Commands": [
        {
            "Name": "status",
            "Path": "${ServiceRoot}/bin/my-app-cli.exe",
            "Args": ["status", "--verbose"]
        }
    ]
}
```

### **Configuration Details**

* **Variable Substitution**: `${ServiceName}` and `${ServiceRoot}` are automatically replaced with the service's name and its root directory.  
* **Paths**: All relative paths are based at the service root.  
* **File Globbing**:  If a path contains a glob pattern (e.g. \*) and matches multiple files, the lexical highest file match is always used.  This powerful mechanism can be used to support version selection (See Recommend Versioning Strategy)  
* **Cron Syntax:** Scheduled tasks use a standard 6-field cron syntax (including seconds), which provides fine-grained scheduling control.  
* **MonitorPing URLs**: The `URL` for monitoring supports multiple schemes:  
  * `http(s)://...`: Checks for a `200 OK` status.  
  * `tcp://host:port`: Checks if a TCP connection can be established.  
  * `echo://host:port`: Sends a string and expects the same string back.  
  * `file:///path/to/file`: Checks if the file's modification time or size has changed since the last check.  
* **Includes**: The `Include` paths support glob patterns (e.g., `v*`) to easily load the latest version of a component's configuration.

For more detailed and advanced configuration examples, please see the files in the `conf/examples` directory.

---

## **Auto-Updates**

The `updater` binary provides a powerful and secure way to keep your application up-to-date.

### **Update Flow**

1. The `updater` is called with a URL pointing to a signed update manifest.  
2. It sends its current version (from a local `.version` file) and profile information to the server.  
3. If the server (cloud endpoint) returns a manifest for a newer version, the `updater` validates its digital signature.  
4. It downloads the update package (a `.zip` file) specified in the manifest.  
5. It verifies the package's checksum (SHA256 or SHA1).  
6. It extracts the package contents into the service root.  
7. It executes any post-update `Operations` defined in the manifest.  
8. It writes the new version number to the `.version` file.  
9. Finally, it creates a `.reload` file, signalling the main `service` to perform a graceful restart and load the new version.

### **The Update Manifest**

The server should return a JSON manifest like this. If you are using signed manifests, the `jsonsig` tool will add the `signature` field automatically (See Signing Manifests).

```
{
  "Version": "2.1.0",
  "URL": "https://updates.example.com/myapp/v2.1.0/myapp-v2.1.0.zip",
  "Sha256": "a1b2c3d4e5f6...",
  "Operations": [
    {
      "Action": "remove",
      "Args": ["data/old-file.exe"]
    },
    {
      "Action": "move",
      "Args": ["temp-v2025-03-25-2.1.0", "v2025-03-25-2.1.0"]
    },
    {
      "Action": "exec",
      "Args": ["v2025-03-25-2.1.0/post-update-hook.bat"]
    }
  ]
}
```

### **Manifest Operations**

You can define a series of operations to run after the update is extracted:

* `exec` / `run`: Execute a command.  
* `copy` / `cp`: Copy a file or directory.  
* `move` / `mv`: Move/rename a file or directory.  
* `remove` / `rm` / `del`: Delete a file or directory.  
* `batchrename`: Recursively find and rename files in a directory.

## **A Robust Upgrade Strategy**

Overwriting files in-place during an upgrade is risky. A partial update caused by a full disk, an inconveniently timed system reboot, or a permissions issue can leave your application in an unrecoverable state.

Silver is designed to support a much more robust, atomic upgrade strategy that leverages versioned directories, path globbing, and a final atomic `move` operation.

### **The Atomic Upgrade Process**

This process ensures that a new version is only activated once it's fully on disk and validated, making your upgrades safe and reliable.

1. **Package Correctly**: In your build process, package all new release files inside a uniquely named root directory within your zip file. A good practice is to prefix it with `temp-`, for example: `temp-v2025-08-15`.  
2. **Download and Extract**: The `updater` downloads and extracts the zip file. This creates the `temp-v2025-08-15/` directory on disk, containing the full new version of your application.  
3. **Execute Operations**: After a successful extraction and checksum validation, the `updater` runs the `Operations` from the update manifest.  
4. **Activate with an Atomic Move**: A key operation is an atomic `move`, which renames the temporary directory to its final versioned name, like `v2025-08-15`. This is a single, near-instantaneous filesystem operation that makes the new version "live".

```
  {
    "Action": "move",
    "Args": ["temp-v*", "v2025-08-15"]
  }
```

5. **Auto-Select on Restart**: Silver's main configuration file should point to your application binary using a glob pattern (a wildcard). When the service restarts, this glob pattern will automatically select the executable from the latest versioned directory, because `v2025-08-15` sorts lexically after `v2025-07-22`.
```
  "Services": [
    {
      "Path": "${ServiceRoot}/v*/my-app-server.exe"
    }
  ]
```

   

### **Version Directory Naming**

For this strategy to work, the directory names for new versions **must sort lexically after older versions**. Here are three recommended conventions:

* **Reverse ISO Date**: A timestamp from the time of the release. This is simple and guarantees correct ordering.  
  * e.g. `v2025-08-15, or v2025-08-15-094500`  
* **Zero-Padded Integer**: A simple, incrementing build number. The padding is crucial for correct lexical sorting (e.g., so that `v010` correctly comes after `v009`).  
  * e.g. `v00001`, `v00002`  
* **Hybrid Version**: Combine a sortable prefix with your human-readable semantic version.  
  * e.g. `v00025-1.1.14`

### **Example Directory Structure**

A typical installation using this strategy might look like this:

```
C:\Program Files\My App\
├── my-app.exe                  <-- The Silver service binary
├── my-app.conf                 <-- The Silver JSON config
├── my-app.log                  <-- The consolidated log file
├── updater.exe                 <-- The Silver updater binary
├── data/                       <-- App data that will remain 
│                                   consistent between versions
│                                   (e.g. database, config, etc.)
├── v00001/                     <-- An old version directory
│   └── my-app-microservice.exe
└── v00002/                     <-- The current, active version 
    └── my-app-microservice.exe

```

---

### **Best Practices**

* **Randomize Update Checks:** Use `StartupRandomDelaySecs` on scheduled tasks to prevent overwhelming your server with simultaneous requests (the "thundering herd" problem). Adding a random delay, say by 1 hour, spreads out tasks like update checks, reducing peak load.  
* **Pre-flight Checks**: Before the final atomic `move`, you can run a validation step. Add an `exec` operation that runs a test command in your new binary (e.g., `my-app-server.exe --test`). If the command fails (returns a non-zero exit code), the entire upgrade process will abort, preventing a broken version from being activated.  
* **Cleaning Up Old Versions**: To prevent disk space from growing indefinitely, you should periodically clean up old version directories. This can be done with a `remove` operation in your update manifest. For complex logic (e.g., "remove all but the last 3 versions"), it's most reliable to use an `exec` operation that calls a small cleanup script/program.

```
// Example: remove all versions from 2024
{
  "Action": "remove",
  "Args": ["v2024-*"]
}
```

### **Advanced: Component-Based Upgrades**

If your application consists of multiple components or microservices on different release schedules (e.g. different engineering teams), you can extend this pattern. Each component can live in its own subdirectory and be updated independently.

The main `my-app.conf` can run multiple `updater` tasks, one for each component.

```
C:\Program Files\My App\
├── my-app.exe
├── my-app.conf         <-- Main config, includes conf files from components
├── updater.exe
├── component1/
│   ├── v00005/
│   │   ├── component1-server.exe
│   │   └── component1-silver-include.conf  <-- Config for this component
│   └── updater-c1.exe  <-- A dedicated updater for component 1
└── component2/
    ├── v00029/
    │   ├── component2-server.exe
    │   └── component2-silver-include.conf
    └── updater-c2.exe
```

By using the `Include` directive in `my-app.conf` to load the `*.conf` files from the components' versioned directories, you allow each component team to manage their own service definitions, scheduled tasks, and other configurations. This configuration is then deployed atomically with their binaries, providing excellent isolation and team autonomy.

### **Security: Signing Manifests**

While delivering manifests over a secure HTTPS connection is a fundamental first step, Silver also supports **end-to-end security via signed manifests**. This protects against a compromised server by ensuring the update payload is authentic and can even secure updates in non-HTTPS environments. For this purpose, Silver includes a command-line utility, `jsonsig`, for this purpose. It uses an Ed25519 public/private key pair.

1. **Generate a key pair:**  
```
  # This creates priv.key (keep it secret!) and pub.key (distribute with your app)
  jsonsig generate --private-key=priv.key --public-key=pub.key
```

2. **Sign your manifest:**  
```
  # This adds the "signature" field to your manifest
  jsonsig sign --private-key=priv.key --input=manifest.json --output=signed-manifest.json
```

3. **Configure the updater:** In your `service.conf`, provide the base64-encoded public key to the `updater` via the `--public-key` flag. The updater will refuse any unsigned or invalid manifest.  For example, your updater task in `service.conf` would look like this:
```
   {  
      "Schedule": "0 0 13 * * *",  
      "Path": "${ServiceRoot}/updater.exe",  
      "Args": ["https://updates.example.com/mycoolapp/version-manifest.json", "--public-key=m7kb8SVfRMFcCVqm18/c+lMd5TS2btIpEhGCZa5VgrI="], 
      "StartupRandomDelaySecs": 3600, 
      "TimeoutSecs": 3600 
   }
```

---

## **Command-Line Interface**

### **Service (`service.exe`)**

* `service.exe install`: Installs the application as a system service.  
* `service.exe uninstall`: Removes the service.  
* `service.exe start`: Starts the service.  
* `service.exe stop`: Stops the service.  
* `service.exe run`: Runs the application in the foreground (useful for debugging).  
* `service.exe validate`: Parses and validates the configuration file.  
* `service.exe command <command-name> [args...]`: Executes a command defined in the `Commands` section of the config.

### **Updater (`updater.exe`)**

* `updater.exe [update-url] --public-key=...`: Checks for and performs an update.  
* `updater.exe -v`: Displays the current version from the `.version` file.  
* `updater.exe profile-set-random-id`: Sets a unique random ID for this installation, sent to the update server.  
* `updater.exe profile-set-channel <channel-name>`: Sets the update channel (e.g., `beta`, `stable`), also sent to the update server for targeted rollouts.

---

## **Building from Source**

To build the `service` and `updater` binaries:

Bash

```
go run make.go
```

The compiled binaries will be placed in the `build/<os>/` directory. The build script supports cross-compilation via the `-goos` and `-goarch` flags.

Bash

```
# Example: build for 64-bit Windows
go run make.go -goos=windows -goarch=amd64
```

---

## **Go Version Policy**

Silver takes a **conservative approach** to its Go version. This policy is designed to maximise compatibility with the wide range of client operating systems that are actively in use and supported.

The project currently targets **Go 1.20**.

## **Licence**

This project is licensed under the MIT Licence. See the `LICENSE` file for details.

## **About This Project**

Silver is an open-source project actively maintained and supported by PaperCut Software. It is battle-tested technology, used in production to manage server and desktop components for millions of laptops and servers running [PaperCut's print management software](https://www.papercut.com/) for nearly a decade.  Silver is a better tool thanks to the collective effort of its community. A big thank you to everyone who has contributed their time, ideas, and code to the project.

