# Plan

## Test plan

1. [x] register
2. [x] heartbeat

## Problems

1. [ ] Sometimes server stuck, with 1 unfinished subtask
2. [ ] Should be able to cache downloaded tasks.
3. [ ] Should handle client abort.
4. [ ] Should check if user is running daemon
5. [ ] Should calculate speed accurately
6. [ ] Should return how many active agents ??
7. [ ] Should return status of current status -- size of queue, total download size, current speed, estimated wait time.

## Dev plan

1. [ ] client runs as a daemon:
   1. [ ] /var/run/ddson.pid to track PID
   2. [ ] /var/log/ddson.log to save logs
   3. [ ] --daemon to start as daemon
   4. [ ] --stop to stop the daemon
2. [ ] Daemonize using systemd:
   1. [ ] log to `stdout`
   2. [ ] shut down on `SIGTERM` or `SIGINT`
   3. [ ] Reload config on `SIGHUP`
   4. [ ] A config file for systemd system.
3. [ ] handle SIGTERM to shutdown gracefully
4. [ ] move supporting go code to a separate git repository, so they can be shared across project.
5. [ ] fail a subtask if it is too slow

## MISC

Install `Todo Tree` extension
