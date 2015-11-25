# go-fastcgi-client
I've exported from code.google.com/p/go-fastcgi-client and upgraded a little.

Anyway, it's not production-ready, but I'm working on it.

### Example usage ###
    fcgi, err := fcgiclient.New("127.0.0.1", 9000)
    if err != nil {
        log.Fatalf("Ooops, could not connect to fscgi server: %v", err)
    }   

    request, err := http.NewRequest("GET", "http://google.com/php-status?json", nil)
    request.Header["Host"] = append(request.Header["Host"], "google.com")
    if err != nil {
        log.Fatalf("Could not create request: %v", err)
    }   

    http_response, err := fcgi.DoHTTPRequest(request, "/tmp/test.php")
    if err != nil {
        log.Fatalf("err: %v", err)
    }   
    log.Printf("Resp code: %v", http_response.ResponseCode)
    r := bufio.NewReader(http_response.Body)

    for {
        myStr, err := r.ReadString('\n')
        log.Printf("Resp line: %v", myStr)
        if err != nil {
            break
        }
    }


