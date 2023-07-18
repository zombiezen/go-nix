{
  description = "go-nix development environment";

  inputs = {
    nixpkgs.url = "nixpkgs";
    flake-utils.url = "flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }@inputs:
    flake-utils.lib.eachDefaultSystem (system:
    let
      pkgs = import nixpkgs { inherit system; };
    in
    {
      packages.default = pkgs.buildGoModule {
        name = "gonix";

        src = self;
        vendorHash = "sha256-Sktr5h7stUy9n9JazXRAxXzThL4Cq4X+7KDpWhvX8rQ=";

        ldflags = [ "-s" "-w" ];

        subPackages = [ "cmd/gonix" ];
      };

      apps.default = {
        type = "app";
        program = "${self.packages.${system}.default}/bin/gonix";
      };

      devShells.default = pkgs.mkShell {
        packages = [
          pkgs.go-tools
          pkgs.go_1_20
          pkgs.gotools
        ];
      };
    });
}
