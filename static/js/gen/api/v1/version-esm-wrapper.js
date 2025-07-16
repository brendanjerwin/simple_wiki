// ESM wrapper for the generated gRPC-web client
import grpcWeb from 'grpc-web';

// Create a global require function for the generated code
if (typeof window !== 'undefined' && !window.require) {
  window.require = function(module) {
    if (module === 'grpc-web') {
      return grpcWeb;
    }
    if (module === 'google-protobuf/google/protobuf/timestamp_pb.js') {
      return {}; // Mock for now
    }
    throw new Error(`Module ${module} not found`);
  };
}

// Import the generated files (this will use the global require)
import('./version_grpc_web_pb.js').then(grpcModule => {
  window.VersionPromiseClient = grpcModule.VersionPromiseClient;
});

import('./version_pb.js').then(pbModule => {
  window.GetVersionRequest = pbModule.GetVersionRequest;
  window.GetVersionResponse = pbModule.GetVersionResponse;
});

// Export placeholders that will be filled by the dynamic imports
export let VersionPromiseClient;
export let GetVersionRequest;
export let GetVersionResponse;

// Wait for the modules to load
const waitForModules = () => {
  return new Promise((resolve) => {
    const check = () => {
      if (window.VersionPromiseClient && window.GetVersionRequest) {
        VersionPromiseClient = window.VersionPromiseClient;
        GetVersionRequest = window.GetVersionRequest;
        GetVersionResponse = window.GetVersionResponse;
        resolve({ VersionPromiseClient, GetVersionRequest, GetVersionResponse });
      } else {
        setTimeout(check, 10);
      }
    };
    check();
  });
};

export { waitForModules };