import { Request, Client, checkstatus, FileStream } from "k6/x/fasthttp"

const config = {
	max_conns_per_host: 1,
	tls_config: {
		insecure_skip_verify: true,
	}
}

const client = new Client(config);

const binFile = new FileStream('/home/dom/go/src/github.com/domsolutions/gopayloader/gentoo_root.img');

let req = new Request("https://localhost:8080/", {
	headers : { "header1" : "headerval1", "header2" : "headerval2", "Content-Type": "application/octet-stream" },
	body : binFile,
});


// Simulated user behavior
export default function () {
	let res = client.post(req);
	checkstatus(200, res);
}
