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
          golangci-lint = pkgs.golangci-lint.override { buildGoModule = buildGo122Module; };

          go-mod-upgrade = buildGo122Module rec {
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

          setup-envtest = buildGo122Module rec {
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

          goprintconst = buildGo122Module rec {
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

          clusterawsadm = buildGo122Module rec {
            name = "clusterawsadm";
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
            ldflags = let t = "sigs.k8s.io/cluster-api-provider-aws/v2/version"; in [
              "-s"
              "-w"
              "-X" "${t}.gitVersion=v${version}"
              "-X" "${t}.gitCommit=v${version}"
              "-X" "${t}.gitReleaseCommit=v${version}"
              "-X" "${t}.gitMajor=${lib.versions.major version}"
              "-X" "${t}.gitMinor=${lib.versions.minor version}"
              "-X" "${t}.buildDate=19700101-00:00:00"
              "-X" "${t}.gitTreeState=clean"
            ];
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

          controller-gen = buildGo122Module rec {
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

          clusterctl = buildGo122Module rec {
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

          helm-schema = buildGo122Module rec {
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
