{
  description = "Useful flakes for golang and Kubernetes projects";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = inputs @ { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      with nixpkgs.legacyPackages.${system}; rec {
        # clusterctl 1.12.3 (v1beta2-compatible) from CAPI release binaries
        clusterctlBin =
          let
            version = "1.12.3";
            base = "https://github.com/kubernetes-sigs/cluster-api/releases/download/v${version}";
            fetch = name: hash: fetchurl {
              url = "${base}/clusterctl-${name}";
              sha256 = hash;
            };
            bins = {
              aarch64-darwin = fetch "darwin-arm64" "1rg6sl78dhi4n86hyjr2mqlnaqz3bk6hq8ipqn8k25d39r0y7yzk";
              x86_64-darwin = fetch "darwin-amd64" "0ay9k7jwxm8sda1v5bngd9hz74z3cmf1dw85rwcxy3bxx5wlm1i4";
              x86_64-linux = fetch "linux-amd64" "0jbky46sb5hksslqbq3q1k9vzkhj2i4xg2jwwvn64sk52mxjhxcv";
              aarch64-linux = fetch "linux-arm64" "1qg3yncgqvjrffg7ajy5alb1fr11qyyz8rkpv3rlv1jqzi3xyjwp";
            };
            bin = bins.${system} or (throw "clusterctl: unsupported system ${system}");
          in
          runCommand "clusterctl-${version}" { } ''
            mkdir -p $out/bin
            cp ${bin} $out/bin/clusterctl
            chmod +x $out/bin/clusterctl
          '';

        packages = rec {
          clusterctl = clusterctlBin;

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
            version = "2.9.1";
            src = fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "cluster-api-provider-aws";
              rev = "v${version}";
              hash = "sha256-wMk57wBTl7xXp5qTL0KA4crX506WCOV0fvQiRYtsYv4=";
            };
            doCheck = false;
            subPackages = [ "cmd/clusterawsadm" ];
            vendorHash = "sha256-ow208a42dvMDQf8KbI/tFmsZD91qXRi1lFNIMmnPIso=";
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
            version = "17.1.2";
            src = fetchFromGitHub {
              owner = "googleapis";
              repo = "release-please";
              rev = "v${version}";
              hash = "sha256-tyxyyiPE9BkZKLDQATZwySM5qFobBPSGsvYs8gZ2K2k=";
            };
            npmDepsHash = "sha256-NULg1LXGML0J6fEI74hyhT53eFBxpjmyjNn0pIcRApw=";
            dontNpmBuild = true;
          };

          helm-schema = buildGo124Module rec {
            pname = "helm-schema";
            version = "2.3.0";

            src = fetchFromGitHub {
              owner = "losisin";
              repo = "helm-values-schema-json";
              rev = "v${version}";
              hash = "sha256-q5A+tCnuHTtUyejP4flID7XhsoBfWGge2jCgsL0uEOc=";
            };
            doCheck = false;
            vendorHash = "sha256-xmj2i1WNI/9ItbxRk8mPIygjq83xuvNu6THyPqZsysY=";
            ldflags = let t = "main"; in [
              "-s"
              "-w"
              "-X ${t}.BuildDate=19700101-00:00:00"
              "-X ${t}.GitCommit=v${version}"
              "-X ${t}.Version=v${version}"
            ];

            postPatch = ''
              sed -i '/^hooks:/,+2 d' plugin.yaml
              sed -i 's#command: $HELM_PLUGIN_DIR/schema#command: $HELM_PLUGIN_DIR/helm-values-schema-json#' plugin.yaml
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
