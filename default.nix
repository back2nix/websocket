{
  pkgs ? (let
    inherit (builtins) fetchTree fromJSON readFile;
    inherit ((fromJSON (readFile ./flake.lock)).nodes) nixpkgs gomod2nix;
  in
    import (fetchTree nixpkgs.locked) {
      overlays = [(import "${fetchTree gomod2nix.locked}/overlay.nix")];
    }),
  buildGoApplication ? pkgs.buildGoApplication,
}: let
  version = "0.4.0-mvp";

  buildBackend = {
    pname,
    tags ? [],
  }:
    buildGoApplication {
      inherit pname version tags;
      pwd = ./backend;
      src = ./backend;
      vendorSha256 = null;
      modules = ./backend/gomod2nix.toml;
      doCheck = false;
      ldflags = ["-X github.com/back2nix/devils/internal/config.Version=${version}"];
      postInstall = ''
        mkdir -p $out/share/${pname}
        cp -r ${./backend}/certs $out/share/${pname}/
        cp -r ${./backend}/migrations $out/share/${pname}/
      '';
    };
  simple-app = pkgs.buildGoModule {
    pname = "simple-app";
    version = "0.1.0";
    src = ./simple-app;
    vendorHash = null;

    doCheck = false;
    # Меняем название бинарного файла
    postInstall = ''
      mkdir -p $out/bin
      mv $out/bin/simple $out/bin/simple-app
    '';
  };
in rec {
  backend = buildBackend {pname = "web-backend";};

  backend-dev = buildBackend {
    pname = "web-backend-dev";
    tags = ["dev"];
  };

  frontend = pkgs.mkYarnPackage rec {
    pname = "web-frontend";
    inherit version;
    src = ./frontend;

    offlineCache = pkgs.fetchYarnDeps {
      yarnLock = src + "/yarn.lock";
      sha256 = "sha256-p98hZqC7jiq26ntnX2dk0Y5C9eNqCjTm2/b92jS2yPE=";
    };

    configurePhase = ''
      export HOME=$(mktemp -d)
      cp -r $node_modules node_modules
      chmod +w node_modules
    '';

    buildPhase = ''
      sed -i 's/"version": "[^"]*"/"version": "'${version}'"/' package.json
      #echo "export const VERSION = '${version}';" > version.ts
      yarn --offline build
      #sed -i 's/__APP_VERSION__/${version}>/g' dist/index.html
      #sed -i 's/<html lang="en">/<html lang="en" data-version="${version}">/' dist/index.html
      #sed -i 's/meta name="version"/meta name="version" content="${version}"/' dist/index.html
      #sed -i 's|src="/src/main.ts"|src="/src/main.ts?v=${version}"|' dist/index.html
    '';

    installPhase = ''
      mkdir -p $out
      mv dist $out/
    '';

    distPhase = "true";
  };

  full-app = pkgs.stdenv.mkDerivation {
    pname = "full-app";
    inherit version;

    buildInputs = [pkgs.makeWrapper];

    phases = ["installPhase"];

    installPhase = ''
      mkdir -p $out/bin $out/share/web
      cp ${backend}/bin/* $out/bin/
      cp -r ${frontend}/dist $out/share/web/static
      cp -r ${backend}/share/web-backend/* $out/share/web/

      makeWrapper $out/bin/web-backend $out/bin/full-app \
        --set STATIC_FILES_PATH $out/share/web/static \
        --set MIGRATIONS_PATH $out/share/web/migrations \
        --set HTTPS_CERT $out/share/web/certs/server.crt \
        --set HTTPS_KEY $out/share/web/certs/server.key
    '';
  };

  # full-app-dev = pkgs.stdenv.mkDerivation {
  #   pname = "full-app-dev";
  #   inherit version;

  #   buildInputs = [pkgs.makeWrapper];

  #   phases = ["installPhase"];

  #   installPhase = ''
  #     mkdir -p $out/bin $out/share/web
  #     cp ${backend-dev}/bin/* $out/bin/
  #     cp -r ${frontend}/dist $out/share/web/static

  #     makeWrapper $out/bin/web-backend-dev $out/bin/full-app-dev \
  #       --set STATIC_FILES_PATH $out/share/web/static
  #   '';
  # };
  inherit simple-app;
}
