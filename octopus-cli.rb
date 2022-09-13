class OctopusCliNew < Formula
    desc "The New CLI (octopus) for Octopus Deploy, a user-friendly DevOps tool for developers that supports release management, deployment automation, and operations runbooks"
    homepage "https://github.com/OctopusDeploy/cl"
    version "0.1.0"
    url "https://github.com/OctopusDeploy/cli/releases/download/v0.1.0/octopus_0.1.0_Darwin_x86_64.tar.gz"
    sha256 "b5f303a3d0a20e0b799a4d7882ec1c79a9243e54453c470cd971986f2fa70cfc"
  
    def install
      bin.install "octopus"
    end
  
    test do
      system "#{bin}/octopus", "--version"
    end
  end