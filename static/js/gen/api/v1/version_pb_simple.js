// Simplified protobuf classes for JavaScript compatibility
export class GetVersionRequest {
  constructor() {}
  
  static fromPartial(partial) {
    return new GetVersionRequest();
  }
}

export class GetVersionResponse {
  constructor() {
    this.version = '';
    this.commit = '';
    this.buildTime = null;
  }
  
  static fromPartial(partial) {
    const response = new GetVersionResponse();
    if (partial.version) response.version = partial.version;
    if (partial.commit) response.commit = partial.commit;
    if (partial.buildTime) response.buildTime = partial.buildTime;
    return response;
  }
}