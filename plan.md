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

## TODOs

| #    | Done | Pri     | Task                                                                          |
| ---- | ---- | ------- | ----------------------------------------------------------------------------- |
| 1    | [x]  |         | ~~client runs as a daemon~~                                                   |
| 1.1  | [x]  |         | ~~.   /var/run/ddson.pid to track PID~~                                       |
| 1.2  | [x]  |         | ~~.   /var/log/ddson.log to save logs~~                                       |
| 1.3  | [x]  |         | ~~.   --daemon to start as daemon~~                                           |
| 1.4  | [x]  |         | ~~.   --stop to stop the daemon~~                                             |
| 2    | [ ]  | L       | Daemonize using systemd                                                       |
| 2.1  | [ ]  |         | .   log to `stdout`                                                           |
| 2.2  | [ ]  |         | .   shut down on `SIGTERM` or `SIGINT`                                        |
| 2.3  | [ ]  |         | .   Reload config on `SIGHUP`                                                 |
| 2.4  | [ ]  |         | .   A config file for systemd system                                          |
| 3    | [ ]  | L       | .   handle SIGTERM to shutdown gracefully                                     |
| 4    | [x]  | L       | ~~logging: rotate~~                                                           |
| 5    | [ ]  | L       | move supporting go code to a separate git repository                          |
| 6    | [ ]  |         | fail a subtask if it is too slow (timeout)                                    |
| 7    | [x]  |         | ~~use a db to track saved files, and cache them~~                             |
| 7.1  | [x]  |         | ~~.   remove old cache items~~                                                |
| 8    | [ ]  |         | more commands                                                                 |
| 8.1  | [ ]  |         | .   query (move to pending tasks to DB)                                       |
| 8.2  | [ ]  |         | .   request                                                                   |
| 8.3  | [ ]  |         | .   download                                                                  |
| 8.4  | [ ]  |         | .   status                                                                    |
| 10   | [ ]  |         | split GRPC interfaces to agent and client                                     |
| 14   | [ ]  |         | agent: speed check                                                            |
| 15   | [ ]  |         | agent: vpn detection                                                          |
| 16   | [ ]  |         | agent: multiple task support                                                  |
| 17   | [ ]  |         | web UI                                                                        |
| 23   | [ ]  |         | if a task is pending, periodically update status                              |
| 9    | [ ]  |         | refactor, maybe re-write: split agent to agent and client                     |
| 11   | [ ]  | Testing | refactor: use AgentManager to manage agents                                   |
| 12   | [ ]  | WIP     | refactor: use TaskManager to manage tasks                                     |
| 13   | [ ]  | Testing | refactor/fix: use ErrorHandler to handle errors (per task)                    |
| 18   | [x]  |         | ~~refactor to standard go project layout~~                                    |
| 19   | [ ]  |         | refactor: better structure                                                    |
| 21   | [x]  |         | ~~refactor: error-handler should not be a global singleton~~                  |
| 20   | [ ]  | ^^^     | refactor, maybe re-write: Include errors in GRPC response, use *DownloadError |
| 22   | [ ]  | ^       | a test plan                                                                   |
| 22.1 | [ ]  | ^       | .   UT                                                                        |
| 22.2 | [ ]  | ^       | .   GRPC interface test                                                       |
| 22.3 | [ ]  | ^       | .   Integration test (with a python server as download source)                |
