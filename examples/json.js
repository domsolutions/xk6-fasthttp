import { Request, Client, checkstatus } from "k6/x/fasthttp"

const client = new Client();


let req = new Request("https://dummyjson.com/products/1");

// Simulated user behavior
export default function () {
	let res = client.get(req);
	checkstatus(200, res);
	console.log(res.json())
}
