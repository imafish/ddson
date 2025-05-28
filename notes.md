

## server


## client

1. start as server
  1. register
  2. keep-alive
  3. receive download task
    1. download
    2. report download status
    3. send file content to server

3. (cmd): request_download
  2. send request to server
    server: >>>
    2. FILE_INFO
    3. FILE_CONTENT
    4. STATUS
      1. number of clients
      2. current speed
      3. total size / remaining size
      4. estimated remaining time
    5. ERROR
  3. print status to console
  4. save_file



## server:
1. manage client list
  1. new client
  2. update keep-alive
  3. remove dead client
2. download
  1. check DB
  2. get file info
  3. split file into tasks
  4. distribute tasks to client
  5. collect downloaded data
  6. merge content
  7. check hash
  9. save file to DB
  10. cleanup
  8. update status to requester
  8. send file to requester







in Server, a client has:

1. Register handler thread
2. Task thread (from DownloadRequest) (from different client instance)
   1. Add task to taskList
   2. Wait task to complete and update download progress
   3. Send data back
   4. ??? What if client is dead?
3. Upload thread (from UploadRequest)
   1. if client is in clientList? NO => ignore data, YES =>
   2. if taskID, subtaskID match? NO => ignore data, YES =>
   3. client is BUSY? NO => synchronization error !!!, YES =>
   4. receive data and save to file. successful? NO => mark subtask as fail, cleanup, YES =>
   5. mark subtask as successful.
   6. ALWAYS: mark client idle, client->currentTask = nil; cond.Broadcast() for clientList;
4. Heartbeat check thread
5. Task execution thread
   1. divide current task into subtasks
   2. find an idle client
   3. 
