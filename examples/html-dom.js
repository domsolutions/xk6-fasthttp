import { Request, Client } from "k6/x/fasthttp"

const client = new Client();

let req = new Request('https://stackoverflow.com');

export default function () {
	let res = client.get(req);
	const doc = res.html();
	doc
		.find('link')
		.toArray()
		.forEach(function (item) {
			console.log(item.attr('href'));
		});
}
