# NCMT
**this is a purely investigational implementation. use at your own risk**  
**heavily** influenced by [nmt](https://github.com/lazyledger/nmt) and [rsmt2d](https://github.com/lazyledger/rsmt2d)
Namespaced Coded Merkle Trees is a go implementation of a coded merkle tree as described in this [paper](https://arxiv.org/abs/1910.01247), 
![Coded Merkle Tree](cmt_viz_m_yu_et_al.png)
with the inclusion of namespaces as described in the [LazyLedger paper](https://arxiv.org/abs/1905.09274). 
![namespaced merkle tree](nmt_viz_LL_Mustafa_Al-Bassam.png)

## Observations and Comparisons to rsmt2d and nmt

- NCMT is significantly more limited to the number of namespaces that can be included per block. Both are limited by the RS codec used, but due to rsmt2d square structure, it can include exponentially more.



