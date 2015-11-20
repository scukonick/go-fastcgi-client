# go-fastcgi-client
I've exported from code.google.com/p/go-fastcgi-client and upgraded a little.

Anyway, it's not production-ready, but I'm working on it.

### Example usage ###
    fcgi, err := fcgiclient.New("127.0.0.1", 9000)
    if err != nil {
        log.Fatalf("Ooops, could not connect to fscgi server: %v", err)
    }   

    env := make(map[string]string)
    env["REQUEST_METHOD"] = "GET"
    env["SCRIPT_FILENAME"] = "/Users/ivan/work/test/fcgi/test.php"
    env["SERVER_SOFTWARE"] = "go / fcgiclient "
    env["REMOTE_ADDR"] = "127.0.0.1"
    env["SERVER_PROTOCOL"] = "HTTP/1.1"
    env["REQUEST_URI"] = "/php-status"
    env["DOCUMENT_URI"] = "/php-status"
    env["SCRIPT_NAME"] = "/php-status"
    
    fcgi_response, err := fcgi.Request(env, "") 
    if err != nil {
        log.Fatalf("err: %v", err)
    }   
    stdout_rec := fcgi_response.Stdouts[0]
    http_response, _ := stdout_rec.ParseStdout()
    log.Printf("%v", http_response.ResponseCode)

