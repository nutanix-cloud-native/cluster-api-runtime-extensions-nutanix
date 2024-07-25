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
          golangci-lint = pkgs.golangci-lint.override { buildGoModule = buildGo123Module; };

          govulncheck = pkgs.govulncheck.override { buildGoModule = buildGo123Module; };

          go-mod-upgrade = buildGo123Module rec {
            name = "go-mod-upgrade";
            version = "0.10.0";
            src = fetchFromGitHub {
              owner = "oligot";
              repo = "go-mod-upgrade";
              rev = "v${version}";
              hash = "sha256-BuHyqv0rK1giNiPO+eCx13rJ9L6y2oCDdKW1sJXyFg4=";
            };
            doCheck = false;
            subPackages = [ "." ];
            vendorHash = "sha256-Qx+8DfeZyNSTf5k4juX7+0IXT4zY2LJMuMw3e1HrxBs=";
            ldflags = [ "-s" "-w" "-X" "main.version=v${version}" ];
          };

          setup-envtest = buildGo123Module rec {
            name = "setup-envtest";
            version = "0.18.5";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "controller-runtime";
              rev = "v${version}";
              hash = "sha256-WGq5juv+QET0PHXW8uj9fV8D3NTSey3RqwvLxJDU014=";
            } + "/tools/setup-envtest";
            doCheck = false;
            subPackages = [ "." ];
            vendorHash = "sha256-tFWXROKZ+5rrHdiY3dFHAo5g5TKYfc8HgLSouD7bI+s=";
            ldflags = [ "-s" "-w" ];
          };

          goprintconst = buildGo123Module rec {
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

          clusterctl-aws = buildGo123Module rec {
            name = "clusterctl-aws";
            version = "2.6.1";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "cluster-api-provider-aws";
              rev = "v${version}";
              hash = "sha256-y3tN/OciKNcqvcROmSKPcehW/4+dBWmGESZYjmYbkBo=";
            };
            doCheck = false;
            subPackages = [ "cmd/clusterawsadm" ];
            vendorHash = "sha256-WRKViPMlJVV3uS+TK6Rd4EoEQ5ttdXDoOqoYiuga1WE=";
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
            version = "16.12.0";
            src = fetchFromGitHub {
              owner = "googleapis";
              repo = "release-please";
              rev = "v${version}";
              hash = "sha256-M4wsk0Vpkl6JAOM2BdSu8Uud7XA+iRHAaQOxHLux+VE=";
            };
            npmDepsHash = "sha256-UXWzBUrZCIklITav3VShL+whiWmvLkFw+/i/k0s13k0=";
            dontNpmBuild = true;
          };

          controller-gen = buildGo123Module rec {
            name = "controller-gen";
            version = "0.16.0";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "controller-tools";
              rev = "v${version}";
              hash = "sha256-0JSzk57XkvlP46SdC50zOtcx+XecGmFfkB+8XyGx6Ps=";
            };
            doCheck = false;
            subPackages = [ "./cmd/controller-gen" ];
            vendorHash = "sha256-3p9K08WMqDRHHa9116//3lFeaMtRaipD4LyisaKWV7I=";
            ldflags = [ "-s" "-w" ];
          };

          clusterctl = buildGo123Module rec {
            pname = "clusterctl";
            version = "1.8.1";

            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "cluster-api";
              rev = "v${version}";
              hash = "sha256-Z7cFwR8IUThEd4Te3KHPC8K8v56ymAG7nIM/7pxWq4U=";
            };
            doCheck = false;
            subPackages = [ "cmd/clusterctl" ];
            vendorHash = "sha256-0VVaD1vGIGezgkVCvIhNHmZqVFxFu4UcUUh0wuX2viw=";
            ldflags = let t = "sigs.k8s.io/cluster-api/version"; in [
              "-X ${t}.gitMajor=${lib.versions.major version}"
              "-X ${t}.gitMinor=${lib.versions.minor version}"
              "-X ${t}.gitVersion=v${version}"
            ];
          };

          helm-schema = buildGo123Module rec {
            pname = "helm-schema";
            version = "1.5.2";

            src = fetchFromGitHub {
              owner = "losisin";
              repo = "helm-values-schema-json";
              rev = "v${version}";
              hash = "sha256-5f54Qcz9Gt/d3qZbIrISr46J0yQKhEg886TSFnWuBXw=";
            };
            doCheck = false;
            vendorHash = "sha256-F2mT36aYkLjUZbV5GQH8mNMZjGi/70dTENU2rRhAJq4=";
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

          helm-list-images = buildGo122Module rec {
            pname = "helm-list-images";
            version = "0.11.0";

            src = fetchFromGitHub {
              owner = "d2iq-labs";
              repo = "helm-list-images";
              rev = "v${version}";
              hash = "sha256-AKj2u0Rz0lK/NJJr+QSM4tLkvZigFS5JfMOg5vVZK8Q=";
            };
            doCheck = false;
            vendorHash = "sha256-YLXRs2vqiwakcxjeoYidpakZJhGO4WjKOrm76Feu5c4=";
            ldflags = let t = "k8s.io/component-base/version"; in [
              "-s"
              "-w"
              "-X ${t}/verflag.programName=${pname}"
              "-X ${t}.buildDate=19700101-00:00:00"
              "-X ${t}.gitCommit=v${version}"
              "-X ${t}.gitMajor==${lib.versions.major version}"
              "-X ${t}.gitMinor==${lib.versions.minor version}"
              "-X ${t}.gitVersion=v${version}"
            ];

            postPatch = ''
              sed -i '/^hooks:/,+2 d' plugin.yaml
            '';

            postInstall = ''
              install -dm755 $out/${pname}
              mv $out/bin/ $out/${pname}/
              install -m644 -Dt $out/${pname} plugin.yaml
            '';
          };

          helm-with-plugins = wrapHelm kubernetes-helm {
            plugins = [
              helm-schema
              helm-list-images
            ];
          };
        };

        formatter = alejandra;
      }
    );
}
