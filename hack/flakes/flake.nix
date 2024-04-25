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
            version = "0.17.2";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "controller-runtime";
              rev = "v${version}";
              hash = "sha256-1u4aFmiDLgvx3CRWv+uZdnP4XuVWpQMmZq27CAF12a4=";
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
            version = "2.4.0";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "cluster-api-provider-aws";
              rev = "v${version}";
              hash = "sha256-va11PBwLh0IcMV4kvXqxAOr9owezYdqvoenIaJPWsDo=";
            };
            doCheck = false;
            subPackages = [ "cmd/clusterawsadm" ];
            vendorHash = "sha256-YoIeRVYnQHG9dYGlSmLsDY1ACbAlXB1t063UFhaJG1w=";
            ldflags = [
              "-s"
              "-w"
              "-X" "sigs.k8s.io/cluster-api-provider-aws/v2/version.gitVersion=v${version}"
              "-X" "sigs.k8s.io/cluster-api-provider-aws/v2/version.gitCommit=v${version}"
              "-X" "sigs.k8s.io/cluster-api-provider-aws/v2/version.gitReleaseCommit=v${version}"
              "-X" "sigs.k8s.io/cluster-api-provider-aws/v2/version.gitMajor=${lib.versions.major version}"
              "-X" "sigs.k8s.io/cluster-api-provider-aws/v2/version.gitMinor=${lib.versions.minor version}"
              "-X" "sigs.k8s.io/cluster-api-provider-aws/v2/version.buildDate=19700101-00:00:00"
              "-X" "sigs.k8s.io/cluster-api-provider-aws/v2/version.gitTreeState=clean"
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
            version = "0.0.1-dev";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "controller-tools";
              rev = "8cc57e3c0607f984b4a595a7cc048ed5c54a6a0d";
              hash = "sha256-vB85JQuvCdCe4qeFiS2SU7NM2ZoKQf7usvY9VUKtGUw=";
            };
            doCheck = false;
            subPackages = [ "./cmd/controller-gen" ];
            vendorHash = "sha256-Fu883lUmlG5PrGdJ9TpvGwvewxP+Riq/gvPxZeq1cy4=";
            ldflags = [ "-s" "-w" ];
          };

          clusterctl = buildGo122Module rec {
            pname = "clusterctl";
            version = "1.7.1";

            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "cluster-api";
              rev = "v${version}";
              hash = "sha256-e+Ut6xoNUY9cJjjJ43P0siEa+G1igllcb8veirk1UX0=";
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
