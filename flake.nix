{
  description = "Custom packages for simple_wiki";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        platformInfo = {
          "x86_64-linux" = {
            suffix = "linux_amd64";
            sha256 = "05k5hb94fxac8a0f2agd7y0mil81rmn63nkdmhimmhnvlgrf9lya";
          };
          "aarch64-darwin" = {
            suffix = "darwin_arm64";
            sha256 = "0n18albrm16sfqxmsp67g6ls8pv01vzwcq44c6x5b8fxzkh13fw2";
          };
          "x86_64-darwin" = {
            suffix = "darwin_amd64";
            sha256 = "1xbgg8glavxkj2ypqg4sdxsx5nx6agy16ils9h70s8kl6p86q6rv";
          };
        };

        info = platformInfo.${system} or (builtins.throw "Unsupported system: ${system}");

        bv = pkgs.stdenv.mkDerivation rec {
          pname = "bv";
          version = "0.11.2";

          src = pkgs.fetchurl {
            url = "https://github.com/Dicklesworthstone/beads_viewer/releases/download/v${version}/bv_${version}_${info.suffix}.tar.gz";
            sha256 = info.sha256;
          };

          sourceRoot = ".";

          installPhase = ''
            mkdir -p $out/bin
            cp bv $out/bin/bv
            chmod +x $out/bin/bv
          '';

          meta = with pkgs.lib; {
            description = "The elegant, keyboard-driven terminal interface for the Beads issue tracker";
            homepage = "https://github.com/Dicklesworthstone/beads_viewer";
            license = licenses.mit;
            platforms = [ "x86_64-linux" "aarch64-darwin" "x86_64-darwin" ];
          };
        };
      in
      {
        packages = {
          inherit bv;
          default = bv;
        };
      }
    );
}
