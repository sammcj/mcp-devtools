import { readFile } from "fs";
import { join } from "path";

function handleRequest(req, res) {
    const data = parseBody(req);
    const result = processData(data);
    return sendResponse(res, result);
}

function parseBody(req) {
    return JSON.parse(req.body);
}

function processData(data) {
    return validate(data);
}

function validate(input) {
    return input !== null;
}

const sendResponse = (res, data) => {
    res.json(data);
};

class Router {
    constructor() {
        this.routes = [];
    }

    addRoute(path, handler) {
        this.routes.push({ path, handler });
    }

    dispatch(req, res) {
        const route = this.routes.find(r => r.path === req.path);
        if (route) {
            route.handler(req, res);
        }
    }
}
