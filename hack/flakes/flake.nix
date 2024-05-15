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
            version = "0.9.1";
            src = fetchFromGitHub {
              owner = "oligot";
              repo = "go-mod-upgrade";
              rev = "v${version}";
              hash = "sha256-+C0IMb7MU1fq/P0/tTUNmzznZ1q5M69491pO5yBZlVs=";
            };
            doCheck = false;
            subPackages = [ "." ];
            vendorHash = "sha256-8rbRxtOiKmnf68kjsUCXaZf+MHI1n5aXa91Aneq9SKo=";
            ldflags = [ "-s" "-w" "-X" "main.version=v${version}" ];
          };

          setup-envtest = buildGo122Module rec {
            name = "setup-envtest";
            version = "0.18.2";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "controller-runtime";
              rev = "v${version}";
              hash = "sha256-fQgWwndxzBIi3zsNMYvFDXjetnaQF0NNK+qW8j4Wn/M=";
            } + "/tools/setup-envtest";
            doCheck = false;
            subPackages = [ "." ];
            vendorHash = "sha256-Xr5b/CRz/DMmoc4bvrEyAZcNufLIZOY5OGQ6yw4/W9k=";
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
            version = "2.5.0";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "cluster-api-provider-aws";
              rev = "v${version}";
              hash = "sha256-iR+r8UaaYIWeFuiGikOdMnCJZuNTQeOKhg2cjTZzs+A=";
            };
            doCheck = false;
            subPackages = [ "cmd/clusterawsadm" ];
            vendorHash = "sha256-mbOnD4idQmN2xcDcD0Li7HrJ5ip1se3mNx6ET2znRFI=";
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
            version = "16.10.2";
            src = fetchFromGitHub {
              owner = "googleapis";
              repo = "release-please";
              rev = "v${version}";
              hash = "sha256-5EST9dNB59wZ9NSHx7V8pAZsws0Py3Q73R6MxvS7zFA=";
            };
            npmDepsHash = "sha256-HZAjBF4dH8JTgJrDrXtxJLyAfKKGn9P5fGBSILx00b8=";
            dontNpmBuild = true;
          };

          controller-gen = buildGo122Module rec {
            name = "controller-gen";
            version = "0.15.0";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "controller-tools";
              rev = "v${version}";
              hash = "sha256-TRJW2//UYQMZM19D74O4SA0GnKPAUI2n+dNKIUzqRuw=";
            };
            doCheck = false;
            subPackages = [ "./cmd/controller-gen" ];
            vendorHash = "sha256-6he/zYznnmhmFU2YPRTnWBTLG2nEOZZu9Iks6orMVMs=";
            ldflags = [ "-s" "-w" ];
          };

          clusterctl = buildGo122Module rec {
            pname = "clusterctl";
            version = "1.7.2";

            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "cluster-api";
              rev = "v${version}";
              hash = "sha256-ZZkDc5INjUoNc9zcwbOa9WRIkkLr9bm3mohsSe3tKI4=";
            };
            doCheck = false;
            subPackages = [ "cmd/clusterctl" ];
            vendorHash = "sha256-ALRnccGjPGuAITtuz79Cao95NhvSczAzspSMXytlw+A=";
            ldflags = let t = "sigs.k8s.io/cluster-api/version"; in [
              "-X ${t}.gitMajor=${lib.versions.major version}"
              "-X ${t}.gitMinor=${lib.versions.minor version}"
              "-X ${t}.gitVersion=v${version}"
            ];
          };
        };

        formatter = alejandra;
      }
    );
}
