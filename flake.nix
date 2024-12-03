{
  description = "A Nix flake for Go backend and Node.js frontend";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix.url = "github:nix-community/gomod2nix";
  };
  outputs = {
    self,
    nixpkgs,
    flake-utils,
    gomod2nix,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {
        inherit system;
        overlays = [
          (final: prev: {
            nodejs = prev.nodejs_20;
            yarn = prev.yarn.override {nodejs = final.nodejs;};
          })
          gomod2nix.overlays.default
        ];
        config = {
          allowUnfree = true;
          allowUnfreePredicate = pkg:
            builtins.elem (pkgs.lib.getName pkg) ["mongodb-compass"];
        };
      };
      callPackage =
        pkgs.darwin.apple_sdk_11_0.callPackage or pkgs.callPackage;
    in {
      packages = rec {
      };
      devShells.default =
        callPackage ./shell.nix {inherit (pkgs) mkGoEnv gomod2nix;};
    });
}
