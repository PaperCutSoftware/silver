SILVER
======

A cross-platform service/daemon wrapper with in-build auto-update, crash resilience and more.

**Current in development. Documentation pending.**

Features:

 * Cross platform - Windows (Service), Mac (Launchd), Linux (systemd)
 * Simple way to host a service (just write a command-line program)
 * Automatic service install
 * Automatic resilience and service crash recovery
 * Application status monitoring with auto restart. Monitor via:
   - HTTP status ping
   - TCP socket echo ping
   - TCP open connection ping
   - File change ping
 * Automatic update support with a framework supporting:
   - validation, 
   - signing, 
   - atomic commits
   - pre and post install actions (copy, move, rename, exec)
 * Run startup tasks (again just command-line programs)
 * Run tasks on a cron schedule
 * Advanced task/service control:
   - Graceful shutdown
   - Start delay
   - Option to randomize task time(s) (e.g. ensure update checks don't all arrive at the same time of day)
 * Lots of other stuff:
   - Logging and log rotation
   - Pid file
   - Simple text based configuration in JSON format 

