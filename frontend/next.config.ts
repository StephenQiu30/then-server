import type { NextConfig } from 'next'

const backendOrigin = process.env.THEN_BACKEND_ORIGIN ?? 'http://127.0.0.1:8080'
const apiRoots =
  'auth|users|privacy|wardrobe|outfit-plans|wear-events|consents|media|deletion-requests|health'

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: `/:api(${apiRoots})/:path*`,
        destination: `${backendOrigin}/:api/:path*`,
      },
    ]
  },
}

export default nextConfig
