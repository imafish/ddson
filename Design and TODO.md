

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
