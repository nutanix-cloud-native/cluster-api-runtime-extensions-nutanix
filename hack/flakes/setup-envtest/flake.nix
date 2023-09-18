{
  description = "Manage binaries for envtest";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, utils }:
    utils.lib.eachSystem [
      "x86_64-linux"
      "x86_64-darwin"
      "aarch64-darwin"
    ]
      (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          version = "0.16.2";
        in
        {
          packages = {
            default = pkgs.buildGoModule {
              pname = "setup-envtest";

              inherit version;

              src = pkgs.fetchFromGitHub {
                owner = "kubernetes-sigs";
                repo = "controller-runtime";
                rev = "v${version}";
                # When changing the version, if there is a problem with the hashes, uncomment the following line
                # and re-run the install, then update with the correct hash as output in the error message..
                # hash = pkgs.lib.fakeHash;
                hash = "sha256-lCR408PTwJ6ZbfJQBpjpvGOnUis8w7GM/JUi+QhYhJQ=";
              }+"/tools/setup-envtest";

              # When changing the version, if there is a problem with the hashes, uncomment the following line
              # and re-run the install, then update with the correct hash as output in the error message..
              # vendorHash = pkgs.lib.fakeHash;
              vendorHash = "sha256-ISVGxhFQh4e0eag9Sw0Zj4u1cG0tudZLhJcGdH5tDo4=";

              CGO_ENABLED = 0;

              ldflags = [
                "-s"
                "-w"
              ];
            };
          };
        });
}
