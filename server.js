var http = require('http');

var handleRequest = function(request, response) {
  console.log('Received request for URL: ' + request.url);
  response.writeHead(200);
  response.end('Hello World!');
};
var www = http.createServer(handleRequest);
www.listen(8080);


function freeze(time) {
    const stop = new Date().getTime() + time;
    while(new Date().getTime() < stop);       
}

// console.log("freeze 3s");
// freeze(60000);
// console.log("done");
// process.exit(0);