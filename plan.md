# Plan

## Test plan

1. [x] register
2. [x] heartbeat

## Problems

1. [x] Sometimes server stuck, with 1 unfinished subtask
2. [ ] Should handle client abort.
3. [ ] Should check if user is running daemon
4. [ ] Should calculate speed accurately
5. [ ] Should return status of current status -- size of queue, total download size, current speed, estimated wait time.

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
4. [x] logging: rotate.
5. [ ] move supporting go code to a separate git repository, so they can be shared across project.
6. [ ] fail a subtask if it is too slow (timeout)
7. [x] use a db to track saved files, and cache them.
   1. [x] remove old cache items.
8. [ ] more commands:
   1. [ ] query
      1. [ ] move to pending tasks to DB
   2. [ ] request
   3. [ ] download
   4. [ ] status

## MISC

Install `Todo Tree` extension
