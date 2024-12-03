{
  pkgs ? import <nixpkgs> {config.allowUnfree = true;},
  mkGoEnv,
  gomod2nix,
}: let
  goEnv = mkGoEnv {pwd = ./backend;};
in
  pkgs.mkShell {
    packages = [
      goEnv
      gomod2nix
      pkgs.go
      pkgs.nodejs
      pkgs.yarn
      pkgs.nodePackages.pnpm
      pkgs.node2nix
      pkgs.yarn2nix
      pkgs.gnumake42
      pkgs.air
      pkgs.mongodb-tools
      pkgs.mongodb-compass
      pkgs.mongosh
      pkgs.process-compose
      pkgs.docker-compose
      pkgs.graphviz
      pkgs.killall
      pkgs.openssl
      pkgs.ffmpeg
      pkgs.redis
      pkgs.mc
      pkgs.jq
      pkgs.easyjson
      pkgs.go-mockery
    ];

    shellHook = ''
      echo "Entering the development environment"
      echo "Go version: $(go version)"
      echo "Node.js version: $(node --version)"
      echo "Yarn version: $(yarn --version)"
      echo "PNPM version: $(pnpm --version)"
    '';
  }
