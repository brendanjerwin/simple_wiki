// Simplified Connect service for JavaScript compatibility
import { GetVersionRequest, GetVersionResponse } from "./version_pb_simple.js";

export const Version = {
  typeName: "api.v1.Version",
  methods: {
    getVersion: {
      name: "GetVersion",
      I: GetVersionRequest,
      O: GetVersionResponse,
      kind: "unary",
    },
  }
};