export default {
  multipass: true,
  floatPrecision: 2,
  plugins: [
    {
      name: 'preset-default',
      params: {
        overrides: {
          // Path coordinates round to 2 decimals; a transform matrix must not.
          // Its scale factors are ~0.283, and rounding those is a percentage
          // error on every coordinate they multiply — which is how a earlier
          // pass at precision 1 quietly threw some cards' transforms away.
          convertTransform: { transformPrecision: 6, floatPrecision: 6 },
          convertPathData: { floatPrecision: 2, transformPrecision: 6 },
          cleanupNumericValues: { floatPrecision: 2 },
        },
      },
    },
  ],
};
