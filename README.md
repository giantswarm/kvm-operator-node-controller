# KVM operator node controller

Manages Kubernetes nodes based on kvm-operator (host cluster) state.

This project implementes own stand-alone node controller and provider for kvm-operator.

Controller and specifically provider implemented similar to [Kubernetes "out-of-tree" cloud-controller-managers.](https://kubernetes.io/docs/tasks/administer-cluster/developing-cloud-controller-manager/).

## Prerequisites

## Getting Project

Download the latest release: https://github.com/giantswarm/kvm-operator-node-controller/releases/latest

Clone the git repository: https://github.com/giantswarm/kvm-operator-node-controller.git

Download the latest docker image from here: TBD

### How to build

#### Dependencies

Dependencies are managed using [`glide`](https://github.com/Masterminds/glide) and contained in the `vendor` directory. See `glide.yaml` for a list of libraries this project directly depends on and `glide.lock` for complete information on all external libraries and their versions used.

**Note:** The `vendor` directory is **flattened**. Always use the `--strip-vendor` (or `-v`) flag when working with `glide`.

#### Building the standard way

```nohighlight
go build
```

#### Cross-compiling in a container

Here goes the documentation on compiling for different architectures from inside a Docker container.

## Running kvm-operator-node-controller

- How to use
- What does it do exactly

## Future Development

- TBD

## Contact

- Mailing list: [giantswarm](https://groups.google.com/forum/!forum/giantswarm)
- IRC: #[giantswarm](irc://irc.freenode.org:6667/#giantswarm) on freenode.org
- Bugs: [issues](https://github.com/giantswarm/kvm-operator-node-controller/issues)

## Contributing & Reporting Bugs

See [.github/CONTRIBUTING.md](/giantswarm/example-opensource-repo/blob/master/.github/CONTRIBUTING.md) for details on submitting patches, the contribution workflow as well as reporting bugs.

## License

kvm-operator-node-controller is under the Apache 2.0 license. See the [LICENSE](/giantswarm/example-opensource-repo/blob/master/LICENSE) file for details.
