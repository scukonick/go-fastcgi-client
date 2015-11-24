# go-fastcgi-client
I've exported from code.google.com/p/go-fastcgi-client and upgraded a little.

Anyway, it's not production-ready, but I'm working on it.

### Example usage ###
    fcgi, err := fcgiclient.New("127.0.0.1", 9000)
    if err != nil {
        log.Fatalf("Ooops, could not connect to fscgi server: %v", err)

    }   

    request, err := http.NewRequest("GET", "http://google.com/aphp-status?json", nil)
    request.Header["Host"] = append(request.Header["Host"], "google.com")
    if err != nil {
        log.Fatalf("Could not create request: %v", err)
    }   

    fcgi_response, err := fcgi.DoHTTPRequest(request, "/tmp/test.php")
    if err != nil {
        log.Fatalf("err: %v", err)
    }   
    stdout_rec := fcgi_response.Stdouts[0]
    http_response, _ := stdout_rec.ParseStdout()
    log.Printf("%v", http_response.ResponseCode)


