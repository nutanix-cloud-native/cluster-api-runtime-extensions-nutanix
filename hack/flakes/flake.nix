{
  description = "Useful flakes for golang and Kubernetes projects";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = inputs @ { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      with nixpkgs.legacyPackages.${system}; rec {
        packages = rec {
          goprintconst = buildGo124Module rec {
            name = "goprintconst";
            version = "0.0.1-dev";
            src = fetchFromGitHub {
              owner = "jimmidyson";
              repo = "goprintconst";
              rev = "088aabfbe96447a809a6a742b6ea0a68f601aa43";
              hash = "sha256-s5CM7BRA231Nzjv3F7qJA6ZM1JC6FnGeFiDiiJTPr3E=";
            };
            doCheck = false;
            subPackages = [ "." ];
            vendorHash = null;
            ldflags = [ "-s" "-w" ];
          };

          clusterctl-aws = buildGo124Module rec {
            name = "clusterctl-aws";
            version = "2.7.1";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "cluster-api-provider-aws";
              rev = "v${version}";
              hash = "sha256-l2ZCylr47vRYw/HyYaeKfSvH1Kt9YQPwLoHLU2h+AE4=";
            };
            doCheck = false;
            subPackages = [ "cmd/clusterawsadm" ];
            vendorHash = "sha256-iAheoh9VMSdTVvJzhXZBFpGDoDsGO8OV/sYjDEsf8qw=";
            ldflags = let modPrefix = "sigs.k8s.io/cluster-api-provider-aws/v2";
                          v = "${modPrefix}/version";
                          c = "${modPrefix}/cmd/clusterawsadm/cmd/version"; in [
              "-s"
              "-w"
              "-X" "${v}.gitVersion=v${version}"
              "-X" "${v}.gitCommit=v${version}"
              "-X" "${v}.gitReleaseCommit=v${version}"
              "-X" "${v}.gitMajor=${lib.versions.major version}"
              "-X" "${v}.gitMinor=${lib.versions.minor version}"
              "-X" "${v}.buildDate=19700101-00:00:00"
              "-X" "${v}.gitTreeState=clean"
              "-X" "${c}.CLIName=clusterctl-aws"
            ];
            preInstall = ''
              mv $GOPATH/bin/clusterawsadm $GOPATH/bin/clusterctl-aws
            '';
          };

          release-please = buildNpmPackage rec {
            pname = "release-please";
            version = "16.18.0";
            src = fetchFromGitHub {
              owner = "googleapis";
              repo = "release-please";
              rev = "v${version}";
              hash = "sha256-iY1EblSMCvw6iy8DFJnQRNCST7wycWSV8vdsq+XNpRU=";
            };
            npmDepsHash = "sha256-HDi7dFG/jNsszyvrb7ravVKQ7XO7NegnbX9MITcS1eE=";
            dontNpmBuild = true;
          };

          helm-schema = buildGo124Module rec {
            pname = "helm-schema";
            version = "1.7.0";

            src = fetchFromGitHub {
              owner = "losisin";
              repo = "helm-values-schema-json";
              rev = "v${version}";
              hash = "sha256-P/3EcVBo11XxY+S8FyDiSUPQNfgTTqLDmbbc7Up5LNc=";
            };
            doCheck = false;
            vendorHash = "sha256-mT2A6xXlTFYrA6yNpz9jaa69vdetY/OgjNtTvG4jAYs=";
            ldflags = let t = "main"; in [
              "-s"
              "-w"
              "-X ${t}.BuildDate=19700101-00:00:00"
              "-X ${t}.GitCommit=v${version}"
              "-X ${t}.Version=v${version}"
            ];

            postPatch = ''
              sed -i '/^hooks:/,+2 d' plugin.yaml
              sed -i 's#command: "$HELM_PLUGIN_DIR/schema"#command: "$HELM_PLUGIN_DIR/helm-values-schema-json"#' plugin.yaml
            '';

            postInstall = ''
              install -dm755 $out/${pname}
              mv $out/bin/* $out/${pname}/
              install -m644 -Dt $out/${pname} plugin.yaml
            '';
          };

          helm-with-plugins = wrapHelm kubernetes-helm {
            plugins = [
              helm-schema
            ];
          };
        };

        formatter = alejandra;
      }
    );
}
