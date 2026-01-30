/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  transpilePackages: ['rete', 'rete-react-plugin', 'rete-area-plugin', 'rete-connection-plugin', 'rete-context-menu-plugin', 'rete-render-utils'],
  compiler: {
    styledComponents: true,
  },
};

module.exports = nextConfig;
